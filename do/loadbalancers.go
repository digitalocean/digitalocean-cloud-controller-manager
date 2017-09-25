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
	goctx "context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/context"
)

const (
	// annDOProtocol is the annotation used to specify the default protocol
	// for DO load balancers. For ports specifed in annDOTLSPorts, this protocol
	// is overwritten to https. Options are tcp, http and https. Defaults to tcp
	annDOProtocol = "service.beta.kubernetes.io/do-loadbalancer-protocol"

	// annDOTLSPorts is the annotation used to specify which ports of the loadbalancer
	// should use the https protocol. This is a comma separated list of ports
	// e.g. 443,6443,7443
	annDOTLSPorts = "service.beta.kubernetes.io/do-loadbalancer-tls-ports"

	// annDOTLSPassThrough is the annotation used to specify whether the
	// DO loadbalancer should pass encrypted data to backend droplets.
	// This is optional and defaults to false
	annDOTLSPassThrough = "service.beta.kubernetes.io/do-loadbalancer-tls-passthrough"

	// annDOCertificateID is the annotation specifying the certificate ID
	// used for https protocol. This annoataion is required if annDOTLSPorts
	// is passed
	annDOCertificateID = "service.beta.kubernetes.io/do-loadbalancer-certificate-id"

	// annDOAlgorithm is the annotation specifying which algorithm DO loadbalancer
	// should use. Options are round_robin and least_connections. Defaults
	// to round_robin
	annDOAlgorithm = "service.beta.kubernetes.io/do-loadbalancer-algorithm"

	// defaultActiveTimeout is the number of seconds to wait for a load balancer to
	// reach the active state
	defaultActiveTimeout = 90

	// defaultActiveCheckTick is the number of seconds between load balancer
	// status checks when waiting for activation
	defaultActiveCheckTick = 5

	// statuses for Digital Ocean load balancer
	lbStatusNew     = "new"
	lbStatusActive  = "active"
	lbStatusErrored = "errored"
)

var lbNotFound = errors.New("loadbalancer not found")

// loadbalancers implements cloudprovider.Loadbalancer
type loadbalancers struct {
	client            *godo.Client
	region            string
	lbActiveTimeout   int
	lbActiveCheckTick int
}

// newLoadbalancers returns a type loadbalancer, implementing cloudprovider.Loadbalancer
func newLoadbalancers(client *godo.Client, region string) cloudprovider.LoadBalancer {
	return &loadbalancers{client, region, defaultActiveTimeout, defaultActiveCheckTick}
}

// GetLoadBalancer specifies whether the loadbalancer exists based on the provided service
// if exists, will return loadbalancer status. v1.Service provdied must be treated
// as read only. clusterName is what's specified in the kube-controller-manager
func (l *loadbalancers) GetLoadBalancer(clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	lbName := cloudprovider.GetLoadBalancerName(service)
	lb, err := l.lbByName(context.TODO(), lbName)
	if err != nil {
		if err == lbNotFound {
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

// EnsureLoadBalancer will create a new load balancer or updating existing ones
// Service and Nodes passed in must be treated as read only.
// clusterName is what's specified in the kube-controller-manager
func (l *loadbalancers) EnsureLoadBalancer(clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	lbStatus, exists, err := l.GetLoadBalancer(clusterName, service)
	if err != nil {
		return nil, err
	}

	if !exists {
		lbRequest, err := l.buildLoadBalancerRequest(service, nodes)
		if err != nil {
			return nil, err
		}

		lb, _, err := l.client.LoadBalancers.Create(context.TODO(), lbRequest)
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

	err = l.UpdateLoadBalancer(clusterName, service, nodes)
	if err != nil {
		return nil, err
	}

	lbStatus, exists, err = l.GetLoadBalancer(clusterName, service)
	if err != nil {
		return nil, err
	}

	return lbStatus, nil

}

// UpdateLoadBalancer updates any droplets under the specified loadbalancer
// services and nodes passed in are to be treated as read only
// clusterName is what's specified in the kube-controller-manager
func (l *loadbalancers) UpdateLoadBalancer(clusterName string, service *v1.Service, nodes []*v1.Node) error {
	lbRequest, err := l.buildLoadBalancerRequest(service, nodes)
	if err != nil {
		return err
	}

	lbName := cloudprovider.GetLoadBalancerName(service)
	lb, err := l.lbByName(context.TODO(), lbName)
	if err != nil {
		return err
	}

	_, _, err = l.client.LoadBalancers.Update(context.TODO(), lb.ID, lbRequest)
	return err
}

// EnsureLoadBalancerDeleted deletes the specified loadbalancer if it exists
// returning nil if the loadbalancer specifed either didn't exist or
// was successfully deleted. Services and nodes passed in are to be treated
// as read only. clusterName is what's specified in kube-controller-manager
func (l *loadbalancers) EnsureLoadBalancerDeleted(clusterName string, service *v1.Service) error {
	_, exists, err := l.GetLoadBalancer(clusterName, service)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	ctx := context.TODO()
	lbName := cloudprovider.GetLoadBalancerName(service)

	lb, err := l.lbByName(ctx, lbName)
	if err != nil {
		return err
	}

	_, err = l.client.LoadBalancers.Delete(ctx, lb.ID)
	return err
}

// lbByName gets a DigitalOcean loadbalancer provided it's name.
// returns lbNotFound error if it doesn't exist
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

	return nil, lbNotFound
}

// nodesToDropletID receives a list of Kubernetes nodes and get's all the corresponding droplet IDs.
// This function assumes nodes names match that of the droplet name
func (l *loadbalancers) nodesToDropletIDs(nodes []*v1.Node) ([]int, error) {

	droplets, err := allDropletList(context.TODO(), l.client)

	if err != nil {
		return nil, err
	}

	var dropletIDs []int
	for _, node := range nodes {
		for _, droplet := range droplets {
			if node.Name == droplet.Name {
				dropletIDs = append(dropletIDs, droplet.ID)
				break
			}
		}
	}

	return dropletIDs, nil
}

// buildLoadBalancerRequest builds godo.LoadBalancerRequest provided
// kubernetes service and list of kubernetes nodes.
func (l *loadbalancers) buildLoadBalancerRequest(service *v1.Service, nodes []*v1.Node) (
	*godo.LoadBalancerRequest, error) {
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

	algorithm := getAlgorithm(service)

	return &godo.LoadBalancerRequest{
		Name:            lbName,
		DropletIDs:      dropletIDs,
		Region:          l.region,
		ForwardingRules: forwardingRules,
		HealthCheck:     healthCheck,
		Algorithm:       algorithm,
	}, nil
}

func (l *loadbalancers) waitActive(lbID string) (*godo.LoadBalancer, error) {

	ctx, cancel := goctx.WithTimeout(goctx.TODO(), time.Second*time.Duration(l.lbActiveTimeout))
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

// buildHealthChecks receives a kubernetes service and builds health checks
// used for DO loadbalancers. Although a Kubernetes Service can have many node ports,
// DO Loadbalancers can only take 1 node port so we choose the first node port
// for health checking.
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

// buildForwardingRules will build forwarding rules for DigitalOcean loadbalancers
// based on the given Kubernetes service
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

// getProtocol returns the desired protocol reading annotiation annDOProtocol
func getProtocol(service *v1.Service) (string, error) {
	protocol, ok := service.Annotations[annDOProtocol]
	if !ok {
		return "tcp", nil
	}

	if protocol != "tcp" && protocol != "http" && protocol != "https" {
		return "", fmt.Errorf("invalid protocol: %q specifed in annotation: %q", protocol, annDOProtocol)
	}

	return protocol, nil
}

// getTLSPorts reads the set of ports that are set to use TLS
// by reading annotiation annDOTLSPorts
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

// getCertificateID gets the certificate ID to use for forwarding rule
// passed in through annotations annDOCertificateID
func getCertificateID(service *v1.Service) string {
	return service.Annotations[annDOCertificateID]
}

// getTlsPassThrough determines if there should be TLS pass through
// to backend nodes. This is specificed with annotation annDOTlsPassThrough
func getTLSPassThrough(service *v1.Service) bool {
	passThrough, ok := service.Annotations[annDOTLSPassThrough]
	if !ok {
		// this is the DO default
		return false
	}

	passThroughBool, err := strconv.ParseBool(passThrough)
	if err != nil {
		// this is the DO default
		return false
	}

	return passThroughBool
}

// getAlgorithm will get the desired algorithm by reading
// an annotation from a service. Defaults to round_robin if
// annotation doesn't exist
func getAlgorithm(service *v1.Service) string {
	algo := service.Annotations[annDOAlgorithm]

	switch algo {
	case "least_connections":
		return "least_connections"
	default:
		return "round_robin"
	}
}
