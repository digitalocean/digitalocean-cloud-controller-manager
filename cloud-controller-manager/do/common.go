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

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
)

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
func nodeAddresses(droplet *godo.Droplet) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: droplet.Name})

	privateIP, err := droplet.PrivateIPv4()
	if err != nil || privateIP == "" {
		return nil, fmt.Errorf("could not get private ip: %v", err)
	}
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: privateIP})

	publicIP, err := droplet.PublicIPv4()
	if err != nil || publicIP == "" {
		return nil, fmt.Errorf("could not get public ip: %v", err)
	}
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: publicIP})

	return addresses, nil
}
