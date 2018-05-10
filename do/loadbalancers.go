/*
Copyright 2017 DigitalOcean

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
	"strconv"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/digitalocean/godo"
	"github.com/golang/glog"
)

const (
	// annDOProtocol is the annotation used to specify the default protocol
	// for DO load balancers. For ports specified in annDOTLSPorts, this protocol
	// is overwritten to https. Options are tcp, http and https. Defaults to tcp.
	annDOProtocol = "service.beta.kubernetes.io/do-loadbalancer-protocol"

	// annDOTLSPorts is the annotation used to specify which ports of the loadbalancer
	// should use the https protocol. This is a comma separated list of ports
	// (e.g. 443,6443,7443).
	annDOTLSPorts = "service.beta.kubernetes.io/do-loadbalancer-tls-ports"

	// annDOTLSPassThrough is the annotation used to specify whether the
	// DO loadbalancer should pass encrypted data to backend droplets.
	// This is optional and defaults to false.
	annDOTLSPassThrough = "service.beta.kubernetes.io/do-loadbalancer-tls-passthrough"

	// annDOCertificateID is the annotation specifying the certificate ID
	// used for https protocol. This annoataion is required if annDOTLSPorts
	// is passed.
	annDOCertificateID = "service.beta.kubernetes.io/do-loadbalancer-certificate-id"

	// annDOAlgorithm is the annotation specifying which algorithm DO loadbalancer
	// should use. Options are round_robin and least_connections. Defaults
	// to round_robin.
	annDOAlgorithm = "service.beta.kubernetes.io/do-loadbalancer-algorithm"

	// annDOStickySessionsType is the annotation specifying which sticky session type
	// DO loadbalancer should use. Options are none and cookies. Defaults
	// to none.
	annDOStickySessionsType = "service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type"

	// annDOStickySessionsCookieName is the annotation specifying what cookie name to use for
	// DO loadbalancer sticky session. This annotation is required if
	// annDOStickySessionType is set to cookies.
	annDOStickySessionsCookieName = "service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-cookie-name"

	// annDOStickySessionsCookieTTL is the annotation specifying TTL of cookie used for
	// DO loadbalancer sticky session. This annotation is required if
	// annDOStickySessionType is set to cookies.
	annDOStickySessionsCookieTTL = "service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-cookie-ttl"

        // annDORedirectHttpToHttps is the annotation specifying whether or not Http traffic
        // should be redirected to Https. Defaults to false
        annDORedirectHttpToHttps = "service.beta.kubernetes.io/do-loadbalancer-redirect-http-to-https"

	// defaultActiveTimeout is the number of seconds to wait for a load balancer to
	// reach the active state.
	defaultActiveTimeout = 90

	// defaultActiveCheckTick is the number of seconds between load balancer
	// status checks when waiting for activation.
	defaultActiveCheckTick = 5

	// statuses for Digital Ocean load balancer
	lbStatusNew     = "new"
	lbStatusActive  = "active"
	lbStatusErrored = "errored"
)

var errLBNotFound = errors.New("loadbalancer not found")

type loadbalancers struct {
	client            *godo.Client
	region            string
	lbActiveTimeout   int
	lbActiveCheckTick int
}

// newLoadbalancers returns a cloudprovider.LoadBalancer whose concrete type is a *loadbalancer.
func newLoadbalancers(client *godo.Client, region string) cloudprovider.LoadBalancer {
	return &loadbalancers{client, region, defaultActiveTimeout, defaultActiveCheckTick}
}

// GetLoadBalancer returns the *v1.LoadBalancerStatus of service.
//
// GetLoadBalancer will not modify service.
func (l *loadbalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	lbName := cloudprovider.GetLoadBalancerName(service)
	lb, err := l.lbByName(ctx, lbName)
	if err != nil {
		if err == errLBNotFound {
			return nil, false, nil
		}

		return nil, false, err
	}

	if lb.Status != lbStatusActive {
		lb, err = l.waitActive(lb.ID)
		if err != nil {
			return nil, true, fmt.Errorf("error waiting for load balancer to be active %v", err)
		}
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.IP,
			},
		},
	}, true, nil
}

// EnsureLoadBalancer ensures that the cluster is running a load balancer for
// service.
//
// EnsureLoadBalancer will not modify service or nodes.
func (l *loadbalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	lbStatus, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return nil, err
	}

	if !exists {
		lbRequest, err := l.buildLoadBalancerRequest(service, nodes)
		if err != nil {
			return nil, err
		}

		lb, _, err := l.client.LoadBalancers.Create(ctx, lbRequest)
		if err != nil {
			return nil, err
		}

		lb, err = l.waitActive(lb.ID)
		if err != nil {
			return nil, err
		}

		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: lb.IP,
				},
			},
		}, nil
	}

	err = l.UpdateLoadBalancer(ctx, clusterName, service, nodes)
	if err != nil {
		return nil, err
	}

	lbStatus, exists, err = l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return nil, err
	}

	return lbStatus, nil

}

// UpdateLoadBalancer updates the load balancer for service to balance across
// the droplets in nodes.
//
// UpdateLoadBalancer will not modify service or nodes.
func (l *loadbalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	lbRequest, err := l.buildLoadBalancerRequest(service, nodes)
	if err != nil {
		return err
	}

	lbName := cloudprovider.GetLoadBalancerName(service)
	lb, err := l.lbByName(ctx, lbName)
	if err != nil {
		return err
	}

	_, _, err = l.client.LoadBalancers.Update(ctx, lb.ID, lbRequest)
	return err
}

// EnsureLoadBalancerDeleted deletes the specified loadbalancer if it exists.
// nil is returned if the load balancer for service does not exist or is
// successfully deleted.
//
// EnsureLoadBalancerDeleted will not modify service.
func (l *loadbalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	_, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	lbName := cloudprovider.GetLoadBalancerName(service)

	lb, err := l.lbByName(ctx, lbName)
	if err != nil {
		return err
	}

	_, err = l.client.LoadBalancers.Delete(ctx, lb.ID)
	return err
}

// lbByName gets a DigitalOcean Load Balancer by name. The returned error will
// be lbNotFound if the load balancer does not exist.
func (l *loadbalancers) lbByName(ctx context.Context, name string) (*godo.LoadBalancer, error) {
	lbs, _, err := l.client.LoadBalancers.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, lb := range lbs {
		if lb.Name == name {
			return &lb, nil
		}
	}

	return nil, errLBNotFound
}

// nodesToDropletID returns a []int containing ids of all droplets identified by name in nodes.
//
// Node names are assumed to match droplet names.
func (l *loadbalancers) nodesToDropletIDs(nodes []*v1.Node) ([]int, error) {
	droplets, err := allDropletList(context.TODO(), l.client)

	if err != nil {
		return nil, err
	}

	var dropletIDs []int
	for _, node := range nodes {
	Loop:
		for _, droplet := range droplets {
			if node.Name == droplet.Name {
				dropletIDs = append(dropletIDs, droplet.ID)
				break
			}
			addresses, err := nodeAddresses(&droplet)
			if err != nil {
				glog.Errorf("error getting node addresses for %s: %v", droplet.Name, err)
				continue
			}
			for _, address := range addresses {
				if address.Address == string(node.Name) {
					dropletIDs = append(dropletIDs, droplet.ID)
					break Loop
				}
			}
		}
	}

	return dropletIDs, nil
}

// buildLoadBalancerRequest returns a *godo.LoadBalancerRequest to balance
// requests for service across nodes.
func (l *loadbalancers) buildLoadBalancerRequest(service *v1.Service, nodes []*v1.Node) (*godo.LoadBalancerRequest, error) {
	lbName := cloudprovider.GetLoadBalancerName(service)

	dropletIDs, err := l.nodesToDropletIDs(nodes)
	if err != nil {
		return nil, err
	}

	forwardingRules, err := buildForwardingRules(service)
	if err != nil {
		return nil, err
	}

	healthCheck, err := buildHealthCheck(service)
	if err != nil {
		return nil, err
	}

	stickySessions, err := buildStickySessions(service)
	if err != nil {
		return nil, err
	}

	algorithm := getAlgorithm(service)

        redirectHttpToHttps := getRedirectHttpToHttps(service)

	return &godo.LoadBalancerRequest{
		Name:                   lbName,
		DropletIDs:             dropletIDs,
		Region:                 l.region,
		ForwardingRules:        forwardingRules,
		HealthCheck:            healthCheck,
		StickySessions:         stickySessions,
		Algorithm:              algorithm,
                RedirectHttpToHttps:    redirectHttpToHttps,
	}, nil
}

func (l *loadbalancers) waitActive(lbID string) (*godo.LoadBalancer, error) {

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*time.Duration(l.lbActiveTimeout))
	defer cancel()
	ticker := time.NewTicker(time.Second * time.Duration(l.lbActiveCheckTick))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			lb, _, err := l.client.LoadBalancers.Get(ctx, lbID)
			if err != nil {
				return nil, err
			}

			if lb.Status == lbStatusActive {
				return lb, nil
			}
			if lb.Status == lbStatusErrored {
				return nil, fmt.Errorf("error creating DigitalOcean balancer: %q", lbID)
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("load balancer creation for %q timed out", lbID)
		}
	}
}

// buildHealthChecks returns a godo.HealthCheck for service.
//
// Although a Kubernetes Service can have many node ports, DigitalOcean Load
// Balancers can only take one node port so we choose the first node port for
// health checking.
func buildHealthCheck(service *v1.Service) (*godo.HealthCheck, error) {
	protocol, err := getProtocol(service)
	if err != nil {
		return nil, err
	}

	port := service.Spec.Ports[0].NodePort

	return &godo.HealthCheck{
		Protocol:               protocol,
		Port:                   int(port),
		CheckIntervalSeconds:   3,
		ResponseTimeoutSeconds: 5,
		HealthyThreshold:       5,
		UnhealthyThreshold:     3,
	}, nil
}

// buildForwardingRules returns the forwarding rules of the Load Balancer of
// service.
func buildForwardingRules(service *v1.Service) ([]godo.ForwardingRule, error) {
	protocol, err := getProtocol(service)
	if err != nil {
		return nil, err
	}

	tlsPorts, err := getTLSPorts(service)
	if err != nil {
		return nil, err
	}

	certificateID := getCertificateID(service)
	tlsPassThrough := getTLSPassThrough(service)

	if len(tlsPorts) > 0 {
		if certificateID == "" && !tlsPassThrough {
			return nil, errors.New("must set certificate id or enable tls pass through")
		}

		if certificateID != "" && tlsPassThrough {
			return nil, errors.New("either certificate id should be set or tls pass through enabled, not both")
		}
	}

	var forwardingRules []godo.ForwardingRule
	for _, port := range service.Spec.Ports {
		var forwardingRule godo.ForwardingRule
		if port.Protocol != "TCP" {
			return nil, fmt.Errorf("only TCP protocol is supported, got: %q", port.Protocol)
		}

		forwardingRule.EntryProtocol = protocol
		forwardingRule.TargetProtocol = protocol

		forwardingRule.EntryPort = int(port.Port)
		forwardingRule.TargetPort = int(port.NodePort)

		if forwardingRule.EntryProtocol == "tcp" {
			forwardingRules = append(forwardingRules, forwardingRule)
			continue
		}

		// TLS rules should only apply when default protocol is http or https
		for _, tlsPort := range tlsPorts {
			if port.Port == int32(tlsPort) {
				forwardingRule.EntryProtocol = "https"
				forwardingRule.TlsPassthrough = tlsPassThrough

				if tlsPassThrough {
					forwardingRule.TargetProtocol = "https"
				} else {
					forwardingRule.CertificateID = certificateID
				}
				break
			}
		}

		forwardingRules = append(forwardingRules, forwardingRule)
	}

	return forwardingRules, nil
}

func buildStickySessions(service *v1.Service) (*godo.StickySessions, error) {
	t := getStickySessionsType(service)

	if t == "none" {
		return &godo.StickySessions{
			Type: t,
		}, nil
	}

	name, err := getStickySessionsCookieName(service)
	if err != nil {
		return nil, err
	}

	ttl, err := getStickySessionsCookieTTL(service)
	if err != nil {
		return nil, err
	}

	return &godo.StickySessions{
		Type:             t,
		CookieName:       name,
		CookieTtlSeconds: ttl,
	}, nil
}

// getProtocol returns the desired protocol of service.
func getProtocol(service *v1.Service) (string, error) {
	protocol, ok := service.Annotations[annDOProtocol]
	if !ok {
		return "tcp", nil
	}

	if protocol != "tcp" && protocol != "http" && protocol != "https" {
		return "", fmt.Errorf("invalid protocol: %q specified in annotation: %q", protocol, annDOProtocol)
	}

	return protocol, nil
}

// getTLSPorts returns the ports of service that are set to use TLS.
func getTLSPorts(service *v1.Service) ([]int, error) {
	tlsPorts, ok := service.Annotations[annDOTLSPorts]
	if !ok {
		return nil, nil
	}

	tlsPortsSlice := strings.Split(tlsPorts, ",")

	tlsPortsInt := make([]int, len(tlsPortsSlice))
	for i, tlsPort := range tlsPortsSlice {
		port, err := strconv.Atoi(tlsPort)
		if err != nil {
			return nil, err
		}

		tlsPortsInt[i] = port
	}

	return tlsPortsInt, nil
}

// getCertificateID returns the certificate ID of service to use for fowarding
// rules.
func getCertificateID(service *v1.Service) string {
	return service.Annotations[annDOCertificateID]
}

// getTLSPassThrough returns true if there should be TLS pass through to
// backend nodes.
func getTLSPassThrough(service *v1.Service) bool {
	passThrough, ok := service.Annotations[annDOTLSPassThrough]
	if !ok {
		return false
	}

	passThroughBool, err := strconv.ParseBool(passThrough)
	if err != nil {
		return false
	}

	return passThroughBool
}

// getAlgorithm returns the load balancing algorithm to use for service.
// round_robin is returned when service does not specify an algorithm.
func getAlgorithm(service *v1.Service) string {
	algo := service.Annotations[annDOAlgorithm]

	switch algo {
	case "least_connections":
		return "least_connections"
	default:
		return "round_robin"
	}
}

// getStickySessionsType returns the sticky session type to use for
// loadbalancer. none is returned when a type is not specified.
func getStickySessionsType(service *v1.Service) string {
	t := service.Annotations[annDOStickySessionsType]

	switch t {
	case "cookies":
		return "cookies"
	default:
		return "none"
	}
}

// getStickySessionsCookieName returns cookie name used for
// loadbalancer sticky sessions.
func getStickySessionsCookieName(service *v1.Service) (string, error) {
	name, ok := service.Annotations[annDOStickySessionsCookieName]
	if !ok || name == "" {
		return "", fmt.Errorf("sticky session cookie name not specified, but required")
	}

	return name, nil
}

// getStickySessionsCookieTTL returns ttl for cookie used for
// loadbalancer sticky sessions.
func getStickySessionsCookieTTL(service *v1.Service) (int, error) {
	ttl, ok := service.Annotations[annDOStickySessionsCookieTTL]
	if !ok || ttl == "" {
		return 0, fmt.Errorf("sticky session cookie ttl not specified, but required")
	}

	return strconv.Atoi(ttl)
}

// getRedirectHttpToHttps returns whether or not Http traffic should be redirected
// to Https traffic for the loadbalancer. false is returned if not specified.
func getRedirectHttpToHttps(service *v1.Service) bool {
	redirectHttpToHttps, ok := service.Annotations[annDORedirectHttpToHttps]
	if !ok {
		return false
	}

	redirectHttpToHttpsBool, err := strconv.ParseBool(redirectHttpToHttps)
	if err != nil {
		return false
	}

	return redirectHttpToHttpsBool
}
