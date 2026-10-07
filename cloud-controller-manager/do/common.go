/*
Copyright 2024 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package do

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
)

// IPFamily represents an IP address family for node address discovery.
type IPFamily string

const (
	ipv4Family IPFamily = "ipv4"
	ipv6Family IPFamily = "ipv6"

	doIPAddrFamiliesEnv string = "DO_IP_ADDR_FAMILIES"
)

var ipFamilies []IPFamily

// getIPFamilies returns the configured IP address families.
// When DO_IP_ADDR_FAMILIES is not set or empty, returns the backward-compatible
// default (IPv4 and IPv6 if available).
func getIPFamilies() []IPFamily {
	if ipFamilies != nil {
		return ipFamilies
	}
	return []IPFamily{ipv4Family, ipv6Family}
}

// setIPFamiliesFromEnv parses DO_IP_ADDR_FAMILIES and configures the families.
// Returns an error if any value is invalid.
func setIPFamiliesFromEnv() error {
	v := os.Getenv(doIPAddrFamiliesEnv)
	if v == "" {
		return nil
	}
	var families []IPFamily
	for _, f := range strings.Split(v, ",") {
		f = strings.TrimSpace(f)
		lower := strings.ToLower(f)
		switch lower {
		case string(ipv4Family):
			families = append(families, ipv4Family)
		case string(ipv6Family):
			families = append(families, ipv6Family)
		default:
			return fmt.Errorf("invalid IP family %q in %s (expected ipv4 or ipv6)", f, doIPAddrFamiliesEnv)
		}
	}
	if len(families) == 0 {
		return fmt.Errorf("must specify at least one IP family in %s", doIPAddrFamiliesEnv)
	}
	ipFamilies = families
	return nil
}

// describeIPFamily returns a human-readable description of an IP family.
func describeIPFamily(family IPFamily) string {
	switch family {
	case ipv4Family:
		return "IPv4"
	case ipv6Family:
		return "IPv6"
	default:
		return string(family)
	}
}

// max page size is 200, but choose a smaller value b/c sometimes objects being listed
// are very large and the response gets too big with 200 objects
const apiResultsPerPage = 50

func allDropletList(ctx context.Context, listFunc func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)) ([]godo.Droplet, error) {
	var list []godo.Droplet

	opt := &godo.ListOptions{Page: 1, PerPage: apiResultsPerPage}
	for {
		droplets, resp, err := listFunc(ctx, opt)

		if err != nil {
			return nil, err
		}

		if resp == nil {
			return nil, errors.New("droplets list request returned no response")
		}

		list = append(list, droplets...)

		// if we are at the last page, break out the for loop
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		opt.Page = page + 1
	}

	return list, nil
}

func filterFirewallList(ctx context.Context, client *godo.Client, matchExpectedFirewallName func(godo.Firewall) bool) (*godo.Firewall, *godo.Response, error) {
	opt := &godo.ListOptions{Page: 1, PerPage: apiResultsPerPage}
	var lastResp *godo.Response
	for {
		firewalls, resp, err := client.Firewalls.List(ctx, opt)
		if err != nil {
			return nil, resp, err
		}
		lastResp = resp

		for _, fw := range firewalls {
			if matchExpectedFirewallName(fw) {
				return &fw, resp, nil
			}
		}

		// if we are at the last page, break out the for loop
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, nil, err
		}

		opt.Page = page + 1
	}

	return nil, lastResp, nil
}

func allLoadBalancerList(ctx context.Context, client *godo.Client) ([]godo.LoadBalancer, error) {
	list := []godo.LoadBalancer{}

	opt := &godo.ListOptions{Page: 1, PerPage: apiResultsPerPage}
	for {
		lbs, resp, err := client.LoadBalancers.List(ctx, opt)
		if err != nil {
			return nil, err
		}

		if resp == nil {
			return nil, errors.New("load balancers list request returned no response")
		}

		list = append(list, lbs...)

		// if we are at the last page, break out the for loop
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		opt.Page = page + 1
	}

	return list, nil
}

// nodeAddresses returns a []v1.NodeAddress from droplet.
// The address families are controlled by the DO_IP_ADDR_FAMILIES environment
// variable. When unset, the default is to include all available addresses.
func nodeAddresses(droplet *godo.Droplet) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: droplet.Name})

	for _, family := range getIPFamilies() {
		addr, err := discoverAddress(droplet, family)
		if err != nil {
			return nil, fmt.Errorf("could not get %s addresses: %v", describeIPFamily(family), err)
		}
		addresses = append(addresses, addr...)
	}

	return addresses, nil
}

// discoverAddress discovers node addresses for a given IP family from a droplet.
func discoverAddress(droplet *godo.Droplet, family IPFamily) ([]v1.NodeAddress, error) {
	switch family {
	case ipv4Family:
		privateIP, err := droplet.PrivateIPv4()
		if err != nil || privateIP == "" {
			return nil, fmt.Errorf("could not get private ip: %v", err)
		}
		addrs := []v1.NodeAddress{
			{Type: v1.NodeInternalIP, Address: privateIP},
		}
		publicIPv4, err := droplet.PublicIPv4()
		if err != nil {
			return nil, fmt.Errorf("could not get public ipv4: %v", err)
		}
		if publicIPv4 != "" {
			addrs = append(addrs, v1.NodeAddress{Type: v1.NodeExternalIP, Address: publicIPv4})
		}
		return addrs, nil
	case ipv6Family:
		publicIPv6, err := droplet.PublicIPv6()
		if err != nil {
			return nil, fmt.Errorf("could not get public ipv6: %v", err)
		}
		if publicIPv6 == "" {
			return nil, nil
		}
		return []v1.NodeAddress{
			{Type: v1.NodeExternalIP, Address: publicIPv6},
		}, nil
	default:
		return nil, fmt.Errorf("unknown IP family: %s", family)
	}
}
