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
	"net/http"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"

	"github.com/digitalocean/godo"
)

const (
	// annDOProtocol is the annotation used to specify the default protocol
	// for DO load balancers. For ports specified in annDOTLSPorts, this protocol
	// is overwritten to https. Options are tcp, http and https. Defaults to tcp.
	annDOProtocol = "service.beta.kubernetes.io/do-loadbalancer-protocol"

	// annDOHealthCheckPath is the annotation used to specify the health check path
	// for DO load balancers. Defaults to '/'.
	annDOHealthCheckPath = "service.beta.kubernetes.io/do-loadbalancer-healthcheck-path"

	// annDOHealthCheckProtocol is the annotation used to specify the health check protocol
	// for DO load balancers. Defaults to the protocol used in
	// 'service.beta.kubernetes.io/do-loadbalancer-protocol'.
	annDOHealthCheckProtocol = "service.beta.kubernetes.io/do-loadbalancer-healthcheck-protocol"

	// annDOHealthCheckIntervalSeconds is the annotation used to specify the
	// number of seconds between between two consecutive health checks. The
	// value must be between 3 and 300. Defaults to 3.
	annDOHealthCheckIntervalSeconds = "service.beta.kubernetes.io/do-loadbalancer-healthcheck-check-interval-seconds"

	// annDOHealthCheckResponseTimeoutSeconds is the annotation used to specify the
	// number of seconds the Load Balancer instance will wait for a response
	// until marking a health check as failed. The value must be between 3 and
	// 300. Defaults to 5.
	annDOHealthCheckResponseTimeoutSeconds = "service.beta.kubernetes.io/do-loadbalancer-healthcheck-response-timeout-seconds"

	// annDOHealthCheckUnhealthyThreshold is the annotation used to specify the
	// number of times a health check must fail for a backend Droplet to be
	// marked "unhealthy" and be removed from the pool for the given service.
	// The value must be between 2 and 10. Defaults to 3.
	annDOHealthCheckUnhealthyThreshold = "service.beta.kubernetes.io/do-loadbalancer-healthcheck-unhealthy-threshold"

	// annDOHealthCheckHealthyThreshold is the annotation used to specify the
	// number of times a health check must pass for a backend Droplet to be
	// marked "healthy" for the given service and be re-added to the pool. The
	// value must be between 2 and 10. Defaults to 5.
	annDOHealthCheckHealthyThreshold = "service.beta.kubernetes.io/do-loadbalancer-healthcheck-healthy-threshold"

	// annDOTLSPorts is the annotation used to specify which ports of the load balancer
	// should use the https protocol. This is a comma separated list of ports
	// (e.g. 443,6443,7443).
	annDOTLSPorts = "service.beta.kubernetes.io/do-loadbalancer-tls-ports"

	// annDOTLSPassThrough is the annotation used to specify whether the
	// DO loadbalancer should pass encrypted data to backend droplets.
	// This is optional and defaults to false.
	annDOTLSPassThrough = "service.beta.kubernetes.io/do-loadbalancer-tls-passthrough"

	// annDOCertificateID is the annotation specifying the certificate ID
	// used for https protocol. This annotation is required if annDOTLSPorts
	// is passed.
	annDOCertificateID = "service.beta.kubernetes.io/do-loadbalancer-certificate-id"

	// annDOAlgorithm is the annotation specifying which algorithm DO load balancer
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
	// DO load balancer sticky session. This annotation is required if
	// annDOStickySessionType is set to cookies.
	annDOStickySessionsCookieTTL = "service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-cookie-ttl"

	// annDORedirectHTTPToHTTPS is the annotation specifying whether or not Http traffic
	// should be redirected to Https. Defaults to false
	annDORedirectHTTPToHTTPS = "service.beta.kubernetes.io/do-loadbalancer-redirect-http-to-https"

	// annDOEnableProxyProtocol is the annotation specifying whether PROXY protocol should
	// be enabled. Defaults to false.
	annDOEnableProxyProtocol = "service.beta.kubernetes.io/do-loadbalancer-enable-proxy-protocol"

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

	// This is the DO-specific tag component prepended to the cluster ID.
	tagPrefixClusterID = "k8s"
)

var errLBNotFound = errors.New("loadbalancer not found")

func buildK8sTag(val string) string {
	return fmt.Sprintf("%s:%s", tagPrefixClusterID, val)
}

type loadBalancers struct {
	resources         *resources
	client            *godo.Client
	region            string
	clusterID         string
	lbActiveTimeout   int
	lbActiveCheckTick int
}

// newLoadbalancers returns a cloudprovider.LoadBalancer whose concrete type is a *loadbalancer.
func newLoadBalancers(resources *resources, client *godo.Client, region string) cloudprovider.LoadBalancer {
	return &loadBalancers{
		resources:         resources,
		client:            client,
		region:            region,
		lbActiveTimeout:   defaultActiveTimeout,
		lbActiveCheckTick: defaultActiveCheckTick,
	}
}

// GetLoadBalancer returns the *v1.LoadBalancerStatus of service.
//
// GetLoadBalancer will not modify service.
func (l *loadBalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	lbName := l.GetLoadBalancerName(ctx, clusterName, service)
	lb, found := l.resources.LoadBalancerByName(lbName)
	if !found {
		return nil, false, nil
	}

	if lb.Status != lbStatusActive {
		return nil, true, fmt.Errorf("load-balancer not active, currently %s", lb.Status)
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.IP,
			},
		},
	}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadBalancers) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return getDefaultLoadBalancerName(service)
}

func getDefaultLoadBalancerName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

// EnsureLoadBalancer ensures that the cluster is running a load balancer for
// service.
//
// EnsureLoadBalancer will not modify service or nodes.
func (l *loadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
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
		l.resources.AddLoadBalancer(*lb)
		if lb.Status != lbStatusActive {
			return nil, fmt.Errorf("load-balancer not active, currently %s", lb.Status)
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
func (l *loadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	lbRequest, err := l.buildLoadBalancerRequest(service, nodes)
	if err != nil {
		return err
	}

	lbName := l.GetLoadBalancerName(ctx, clusterName, service)
	lb, found := l.resources.LoadBalancerByName(lbName)
	if !found {
		return errLBNotFound
	}

	lb, _, err = l.client.LoadBalancers.Update(ctx, lb.ID, lbRequest)
	if err != nil {
		return err
	}
	l.resources.AddLoadBalancer(*lb)
	return nil
}

// EnsureLoadBalancerDeleted deletes the specified loadbalancer if it exists.
// nil is returned if the load balancer for service does not exist or is
// successfully deleted.
//
// EnsureLoadBalancerDeleted will not modify service.
func (l *loadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	_, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	lbName := l.GetLoadBalancerName(ctx, clusterName, service)

	lb, found := l.resources.LoadBalancerByName(lbName)
	if !found {
		return nil
	}

	resp, err := l.client.LoadBalancers.Delete(ctx, lb.ID)
	if err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}
	return fmt.Errorf("failed to delete load-balancer, status: %d %s", resp.StatusCode, resp.Status)
}

// nodesToDropletID returns a []int containing ids of all droplets identified by name in nodes.
//
// Node names are assumed to match droplet names.
func (l *loadBalancers) nodesToDropletIDs(nodes []*v1.Node) ([]int, error) {
	droplets := l.resources.Droplets()
	var dropletIDs []int
	for _, node := range nodes {
	Loop:
		for _, droplet := range droplets {
			if node.Name == droplet.Name {
				dropletIDs = append(dropletIDs, droplet.ID)
				break
			}
			addresses, err := nodeAddresses(droplet)
			if err != nil {
				klog.Errorf("error getting node addresses for %s: %v", droplet.Name, err)
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
func (l *loadBalancers) buildLoadBalancerRequest(service *v1.Service, nodes []*v1.Node) (*godo.LoadBalancerRequest, error) {
	lbName := getDefaultLoadBalancerName(service)

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

	redirectHTTPToHTTPS := getRedirectHTTPToHTTPS(service)
	enableProxyProtocol, err := getEnableProxyProtocol(service)
	if err != nil {
		return nil, err
	}

	var tags []string
	if l.resources.clusterID != "" {
		tags = []string{buildK8sTag(l.resources.clusterID)}
	}

	return &godo.LoadBalancerRequest{
		Name:                lbName,
		DropletIDs:          dropletIDs,
		Region:              l.region,
		ForwardingRules:     forwardingRules,
		HealthCheck:         healthCheck,
		StickySessions:      stickySessions,
		Tags:                tags,
		Algorithm:           algorithm,
		RedirectHttpToHttps: redirectHTTPToHTTPS,
		EnableProxyProtocol: enableProxyProtocol,
		VPCUUID:             l.resources.clusterVPCID,
	}, nil
}

// buildHealthChecks returns a godo.HealthCheck for service.
//
// Although a Kubernetes Service can have many node ports, DigitalOcean Load
// Balancers can only take one node port so we choose the first node port for
// health checking.
func buildHealthCheck(service *v1.Service) (*godo.HealthCheck, error) {
	healthCheckProtocol, err := healthCheckProtocol(service)
	if err != nil {
		return nil, err
	}

	// if no health check protocol is specified, use the protocol used for
	// load balancer traffic.
	if healthCheckProtocol == "" {
		protocol, err := getProtocol(service)
		if err != nil {
			return nil, err
		}

		healthCheckProtocol = protocol
	}

	checkIntervalSecs, err := healthCheckIntervalSeconds(service)
	if err != nil {
		return nil, err
	}
	responseTimeoutSecs, err := healthCheckResponseTimeoutSeconds(service)
	if err != nil {
		return nil, err
	}
	unhealthyThreshold, err := healthCheckUnhealthyThreshold(service)
	if err != nil {
		return nil, err
	}
	healthyThreshold, err := healthCheckHealthyThreshold(service)
	if err != nil {
		return nil, err
	}

	healthCheckPath := healthCheckPath(service)
	port := service.Spec.Ports[0].NodePort

	return &godo.HealthCheck{
		Protocol:               healthCheckProtocol,
		Port:                   int(port),
		Path:                   healthCheckPath,
		CheckIntervalSeconds:   checkIntervalSecs,
		ResponseTimeoutSeconds: responseTimeoutSecs,
		UnhealthyThreshold:     unhealthyThreshold,
		HealthyThreshold:       healthyThreshold,
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

	if (certificateID != "" || tlsPassThrough) && len(tlsPorts) == 0 {
		tlsPorts = append(tlsPorts, 443)
	}

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

		// TLS rules should only apply when default protocol is http or https
		for _, tlsPort := range tlsPorts {
			if port.Port == int32(tlsPort) {
				forwardingRule.EntryProtocol = "https"

				if tlsPassThrough {
					forwardingRule.TlsPassthrough = tlsPassThrough
					forwardingRule.TargetProtocol = "https"
				} else {
					forwardingRule.CertificateID = certificateID
					forwardingRule.TargetProtocol = "http"
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

// healthCheckProtocol returns the health check protocol as specified in the service
func healthCheckProtocol(service *v1.Service) (string, error) {
	protocol, ok := service.Annotations[annDOHealthCheckProtocol]
	if !ok {
		return "", nil
	}

	if protocol != "tcp" && protocol != "http" {
		return "", fmt.Errorf("invalid protocol: %q specified in annotation: %q", protocol, annDOProtocol)
	}

	return protocol, nil
}

// getHealthCheckPath returns the desired path for health checking
// health check path should default to / if not specified
func healthCheckPath(service *v1.Service) string {
	path, ok := service.Annotations[annDOHealthCheckPath]
	if !ok {
		return ""
	}

	return path
}

// healthCheckIntervalSeconds returns the health check interval in seconds
func healthCheckIntervalSeconds(service *v1.Service) (int, error) {
	valStr, ok := service.Annotations[annDOHealthCheckIntervalSeconds]
	if !ok {
		return 3, nil
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse health check interval annotation %q: %s", annDOHealthCheckIntervalSeconds, err)
	}

	return val, nil
}

// healthCheckIntervalSeconds returns the health response timeout in seconds
func healthCheckResponseTimeoutSeconds(service *v1.Service) (int, error) {
	valStr, ok := service.Annotations[annDOHealthCheckResponseTimeoutSeconds]
	if !ok {
		return 5, nil
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse health check response timeout annotation %q: %s", annDOHealthCheckResponseTimeoutSeconds, err)
	}

	return val, nil
}

// healthCheckUnhealthyThreshold returns the health check unhealthy threshold
func healthCheckUnhealthyThreshold(service *v1.Service) (int, error) {
	valStr, ok := service.Annotations[annDOHealthCheckUnhealthyThreshold]
	if !ok {
		return 3, nil
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse health check unhealthy threshold annotation %q: %s", annDOHealthCheckUnhealthyThreshold, err)
	}

	return val, nil
}

// healthCheckHealthyThreshold returns the health check healthy threshold
func healthCheckHealthyThreshold(service *v1.Service) (int, error) {
	valStr, ok := service.Annotations[annDOHealthCheckHealthyThreshold]
	if !ok {
		return 5, nil
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse health check healthy threshold annotation %q: %s", annDOHealthCheckHealthyThreshold, err)
	}

	return val, nil
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

// getRedirectHTTPToHTTPS returns whether or not Http traffic should be redirected
// to Https traffic for the loadbalancer. false is returned if not specified.
func getRedirectHTTPToHTTPS(service *v1.Service) bool {
	redirectHTTPToHTTPS, ok := service.Annotations[annDORedirectHTTPToHTTPS]
	if !ok {
		return false
	}

	redirectHTTPToHTTPSBool, err := strconv.ParseBool(redirectHTTPToHTTPS)
	if err != nil {
		return false
	}

	return redirectHTTPToHTTPSBool
}

// getEnableProxyProtocol returns whether PROXY protocol should be enabled.
// False is returned if not specified.
func getEnableProxyProtocol(service *v1.Service) (bool, error) {
	enableProxyProtocolStr, ok := service.Annotations[annDOEnableProxyProtocol]
	if !ok {
		return false, nil
	}

	enableProxyProtocol, err := strconv.ParseBool(enableProxyProtocolStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse proxy protocol flag %q from annotation %q: %s", enableProxyProtocolStr, annDOEnableProxyProtocol, err)
	}

	return enableProxyProtocol, nil
}
