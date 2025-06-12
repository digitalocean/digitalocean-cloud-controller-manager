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
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/api"
	servicehelpers "k8s.io/cloud-provider/service/helpers"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"

	"github.com/digitalocean/godo"
)

const (

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

	// Sticky sessions types.
	stickySessionsTypeNone    = "none"
	stickySessionsTypeCookies = "cookies"

	// Protocol values.
	protocolTCP   = "tcp"
	protocolUDP   = "udp"
	protocolHTTP  = "http"
	protocolHTTPS = "https"
	protocolHTTP2 = "http2"
	protocolHTTP3 = "http3"

	// Port protocol values.
	portProtocolTCP = "TCP"
	portProtocolUDP = "UDP"

	defaultSecurePort = 443

	kubeProxyHealthPort = 10256
)

var (
	errLBNotFound = errors.New("loadbalancer not found")
	defaultLBType = godo.LoadBalancerTypeRegionalNetwork
)

func init() {
	if lbType := os.Getenv(defaultLBTypeEnv); lbType == godo.LoadBalancerTypeRegional || lbType == godo.LoadBalancerTypeRegionalNetwork {
		defaultLBType = lbType
	}
}

func buildK8sTag(val string) string {
	return fmt.Sprintf("%s:%s", tagPrefixClusterID, val)
}

type loadBalancers struct {
	resources         *resources
	region            string
	clusterID         string
	lbActiveTimeout   int
	lbActiveCheckTick int
}

type servicePatcher struct {
	kclient kubernetes.Interface
	base    *v1.Service
	updated *v1.Service
}

func newServicePatcher(kclient kubernetes.Interface, base *v1.Service) servicePatcher {
	return servicePatcher{
		kclient: kclient,
		base:    base.DeepCopy(),
		updated: base,
	}
}

// Patch will submit a patch request for the Service unless the updated service
// reference contains the same set of annotations as the base copied during
// servicePatcher initialization.
func (sp *servicePatcher) Patch(ctx context.Context, err error) error {
	if reflect.DeepEqual(sp.base.Annotations, sp.updated.Annotations) {
		return err
	}
	perr := patchService(ctx, sp.kclient, sp.base, sp.updated)
	return utilerrors.NewAggregate([]error{err, perr})
}

// newLoadbalancers returns a cloudprovider.LoadBalancer whose concrete type is a *loadbalancer.
func newLoadBalancers(resources *resources, region string) cloudprovider.LoadBalancer {
	return &loadBalancers{
		resources:         resources,
		region:            region,
		lbActiveTimeout:   defaultActiveTimeout,
		lbActiveCheckTick: defaultActiveCheckTick,
	}
}

// GetLoadBalancer returns the *v1.LoadBalancerStatus of service.
//
// GetLoadBalancer will not modify service.
func (l *loadBalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	patcher := newServicePatcher(l.resources.kclient, service)
	defer func() { err = patcher.Patch(ctx, err) }()

	var lb *godo.LoadBalancer
	lb, err = l.retrieveAndAnnotateLoadBalancer(ctx, service)
	if err != nil {
		if err == errLBNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	ingress := []v1.LoadBalancerIngress{
		{
			IP: lb.IP,
		},
	}

	if lb.IPv6 != "" {
		ingress = append(ingress, v1.LoadBalancerIngress{
			IP: lb.IPv6,
		})
	}

	return &v1.LoadBalancerStatus{
		Ingress: ingress,
	}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadBalancers) GetLoadBalancerName(_ context.Context, clusterName string, service *v1.Service) string {
	return getLoadBalancerName(service)
}

func getLoadBalancerName(service *v1.Service) string {
	name := service.Annotations[annDOLoadBalancerName]

	if len(name) > 0 {
		return name
	}

	return getLoadBalancerLegacyName(service)
}

func getLoadBalancerLegacyName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

// EnsureLoadBalancer ensures that the cluster is running a load balancer for
// service.
//
// EnsureLoadBalancer will not modify nodes, but will set the load balancer ID annotation on the service if
// it succeeds to create it.
func (l *loadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (lbs *v1.LoadBalancerStatus, err error) {
	lbIsDisowned, err := getDisownLB(service)
	if err != nil {
		return nil, err
	}
	if lbIsDisowned {
		klog.Infof("Short-circuiting EnsureLoadBalancer because service %q is disowned", service.Name)
		return &service.Status.LoadBalancer, nil
	}

	patcher := newServicePatcher(l.resources.kclient, service)
	defer func() { err = patcher.Patch(ctx, err) }()

	var lbRequest *godo.LoadBalancerRequest
	lbRequest, err = l.buildLoadBalancerRequest(ctx, service, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to build load-balancer request: %s", err)
	}

	var lb *godo.LoadBalancer
	lb, err = l.retrieveAndAnnotateLoadBalancer(ctx, service)
	switch err {
	case nil:
		// LB existing
		lb, err = l.updateLoadBalancer(ctx, lb, service, nodes)
		if err != nil {
			return nil, err
		}

	case errLBNotFound:
		// LB missing
		lb, _, err = l.resources.gclient.LoadBalancers.Create(ctx, lbRequest)
		if err != nil {
			logLBInfo("CREATE", lbRequest, 2)
			return nil, fmt.Errorf("failed to create load-balancer: %s", err)
		}
		logLBInfo("CREATE", lbRequest, 2)

		updateServiceAnnotation(service, annDOLoadBalancerID, lb.ID)
		updateServiceAnnotation(service, annDOType, lb.Type)

	default:
		// unrecoverable LB retrieval error
		return nil, err
	}

	if lb.Status == lbStatusNew {
		return nil, api.NewRetryError("load-balancer is currently being created", 15*time.Second)
	}

	if lb.Status != lbStatusActive {
		return nil, fmt.Errorf("load-balancer has unexpected status %q", lb.Status)
	}

	// If a LB hostname annotation is specified, return with it instead of the IP.
	hostname := getHostname(service)
	if hostname != "" {
		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					Hostname: hostname,
				},
			},
		}, nil
	}

	ingress := []v1.LoadBalancerIngress{
		{
			IP: lb.IP,
		},
	}

	if lb.IPv6 != "" {
		ingress = append(ingress, v1.LoadBalancerIngress{
			IP: lb.IPv6,
		})
	}

	return &v1.LoadBalancerStatus{Ingress: ingress}, nil
}

func getCertificateIDFromLB(lb *godo.LoadBalancer) string {
	for _, rule := range lb.ForwardingRules {
		if rule.CertificateID != "" {
			return rule.CertificateID
		}
	}
	return ""
}

// recordUpdatedLetsEncryptCert ensures that when DO LBaaS updates its
// lets_encrypt type certificate associated with a Service that the certificate
// annotation on the Service gets newly-updated certificate ID from the
// Load Balancer.
func (l *loadBalancers) recordUpdatedLetsEncryptCert(ctx context.Context, service *v1.Service, lbCertID, serviceCertID string) error {
	if lbCertID != "" && lbCertID != serviceCertID {
		lbCert, _, err := l.resources.gclient.Certificates.Get(ctx, lbCertID)
		if err != nil {
			respErr, ok := err.(*godo.ErrorResponse)
			if ok && respErr.Response.StatusCode == http.StatusNotFound {
				return nil
			}
			return fmt.Errorf("failed to get DO certificate for load-balancer: %s", err)
		}

		// Prevent unnecessary API call if the LB cert isn't a LE cert.
		if lbCert.Type != certTypeLetsEncrypt {
			return nil
		}

		// Pull the certificate name from the serviceCertID (annotation)
		// This is so that we can determine if the certificate ID has changed
		// due to a LetsEncrypt cert renewal or due to manually changing the certificate
		annCert, _, err := l.resources.gclient.Certificates.Get(ctx, serviceCertID)
		if err != nil {
			respErr, ok := err.(*godo.ErrorResponse)
			if ok && respErr.Response.StatusCode == http.StatusNotFound {
				// something is wrong with the certificate from the service annotation; update it
				updateServiceAnnotation(service, annDOCertificateID, lbCertID)
				return nil
			}
			return fmt.Errorf("failed to get DO certificate for load-balancer: %s", err)
		}

		// Only re-pave the annotation if the certificate names match.
		if lbCert.Name == annCert.Name {
			updateServiceAnnotation(service, annDOCertificateID, lbCertID)
		}

	}

	return nil
}

func (l *loadBalancers) updateLoadBalancer(ctx context.Context, lb *godo.LoadBalancer, service *v1.Service, nodes []*v1.Node) (*godo.LoadBalancer, error) {
	// call buildLoadBalancerRequest for its error checking; we have to call it
	// again just before actually updating the loadbalancer in case
	// checkAndUpdateLBAndServiceCerts modifies the service
	_, err := l.buildLoadBalancerRequest(ctx, service, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to build load-balancer request: %s", err)
	}

	lbCertID := getCertificateIDFromLB(lb)
	serviceCertID, err := findCertificateID(ctx, service, l.resources.gclient)
	if err != nil {
		return nil, err
	}

	err = l.recordUpdatedLetsEncryptCert(ctx, service, lbCertID, serviceCertID)
	if err != nil {
		return nil, err
	}

	lbRequest, err := l.buildLoadBalancerRequest(ctx, service, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to build load-balancer request (post-certificate update): %s", err)
	}

	lbID := lb.ID
	lb, _, err = l.resources.gclient.LoadBalancers.Update(ctx, lb.ID, lbRequest)
	if err != nil {
		logLBInfo("UPDATE", lbRequest, 2)
		return nil, fmt.Errorf("failed to update load-balancer with ID %s: %s", lbID, err)
	}
	logLBInfo("UPDATE", lbRequest, 2)

	return lb, nil
}

// UpdateLoadBalancer updates the load balancer for service to balance across
// the droplets in nodes.
//
// UpdateLoadBalancer will not modify service or nodes.
func (l *loadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (err error) {
	lbIsDisowned, err := getDisownLB(service)
	if err != nil {
		return err
	}
	if lbIsDisowned {
		klog.Infof("Short-circuiting UpdateLoadBalancer because service %q is disowned", service.Name)
		return nil
	}

	patcher := newServicePatcher(l.resources.kclient, service)
	defer func() { err = patcher.Patch(ctx, err) }()

	var lb *godo.LoadBalancer
	lb, err = l.retrieveAndAnnotateLoadBalancer(ctx, service)
	if err != nil {
		return err
	}

	_, err = l.updateLoadBalancer(ctx, lb, service, nodes)
	return err
}

// EnsureLoadBalancerDeleted deletes the specified loadbalancer if it exists.
// nil is returned if the load balancer for service does not exist or is
// successfully deleted.
//
// EnsureLoadBalancerDeleted will not modify service.
func (l *loadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	lbIsDisowned, err := getDisownLB(service)
	if err != nil {
		return err
	}
	if lbIsDisowned {
		klog.Infof("Short-circuiting EnsureLoadBalancerDeleted because service %q is disowned", service.Name)
		return nil
	}

	// Not calling retrieveAndAnnotateLoadBalancer to save a potential PATCH API
	// call: the load-balancer is destined to be removed anyway.
	lb, err := l.retrieveLoadBalancer(ctx, service)
	if err != nil {
		if err == errLBNotFound {
			return nil
		}
		return err
	}

	resp, err := l.resources.gclient.LoadBalancers.Delete(ctx, lb.ID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete load-balancer: %s", err)
	}

	return nil
}

func (l *loadBalancers) retrieveAndAnnotateLoadBalancer(ctx context.Context, service *v1.Service) (*godo.LoadBalancer, error) {
	lb, err := l.retrieveLoadBalancer(ctx, service)
	if err != nil {
		// Return bare error to easily compare for errLBNotFound. Converting to
		// a full error type doesn't seem worth it.
		return nil, err
	}

	updateServiceAnnotation(service, annDOLoadBalancerID, lb.ID)
	updateServiceAnnotation(service, annDOType, lb.Type)

	return lb, nil
}

func (l *loadBalancers) retrieveLoadBalancer(ctx context.Context, service *v1.Service) (*godo.LoadBalancer, error) {
	id := getLoadBalancerID(service)
	if len(id) > 0 {
		klog.V(2).Infof("looking up load-balancer for service %s/%s by ID %s", service.Namespace, service.Name, id)

		return l.findLoadBalancerByID(ctx, id)
	}

	allLBs, err := allLoadBalancerList(ctx, l.resources.gclient)
	if err != nil {
		return nil, err
	}

	lb := findLoadBalancerByName(service, allLBs)
	if lb == nil {
		return nil, errLBNotFound
	}

	return lb, nil
}

func (l *loadBalancers) findLoadBalancerByID(ctx context.Context, id string) (*godo.LoadBalancer, error) {
	lb, resp, err := l.resources.gclient.LoadBalancers.Get(ctx, id)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, errLBNotFound
		}

		return nil, fmt.Errorf("failed to get load-balancer by ID %s: %s", id, err)
	}
	return lb, nil
}

func findLoadBalancerByName(service *v1.Service, allLBs []godo.LoadBalancer) *godo.LoadBalancer {
	customName := getLoadBalancerName(service)
	legacyName := getLoadBalancerLegacyName(service)
	candidates := []string{customName}
	if legacyName != customName {
		candidates = append(candidates, legacyName)
	}

	klog.V(2).Infof("Looking up load-balancer for service %s/%s by name (candidates: %s)", service.Namespace, service.Name, strings.Join(candidates, ", "))

	for _, lb := range allLBs {
		for _, cand := range candidates {
			if lb.Name == cand {
				return &lb
			}
		}
	}

	return nil
}

func findLoadBalancerID(service *v1.Service, allLBs []godo.LoadBalancer) string {
	id := getLoadBalancerID(service)
	if len(id) > 0 {
		return id
	}

	lb := findLoadBalancerByName(service, allLBs)
	if lb == nil {
		return ""
	}

	return lb.ID
}

func updateServiceAnnotation(service *v1.Service, annotName, annotValue string) {
	if service.ObjectMeta.Annotations == nil {
		service.ObjectMeta.Annotations = map[string]string{}
	}
	service.ObjectMeta.Annotations[annotName] = annotValue
}

// nodesToDropletID returns a []int containing ids of all droplets identified by name in nodes.
//
// Node names are assumed to match droplet names.
func (l *loadBalancers) nodesToDropletIDs(ctx context.Context, nodes []*v1.Node) ([]int, error) {
	var dropletIDs []int
	missingDroplets := map[string]bool{}

	for _, node := range nodes {
		providerID := node.Spec.ProviderID
		if providerID != "" {
			dropletID, err := dropletIDFromProviderID(providerID)
			if err != nil {
				return nil, fmt.Errorf("failed to parse provider ID %q: %s", providerID, err)
			}
			dropletIDs = append(dropletIDs, dropletID)
		} else {
			missingDroplets[node.Name] = true
		}
	}

	if len(missingDroplets) > 0 {
		// Discover missing droplets by matching names.
		droplets, err := allDropletList(ctx, l.resources.gclient)
		if err != nil {
			return nil, fmt.Errorf("failed to list all droplets: %s", err)
		}

		for _, droplet := range droplets {
			if missingDroplets[droplet.Name] {
				dropletIDs = append(dropletIDs, droplet.ID)
				delete(missingDroplets, droplet.Name)
				continue
			}
			addresses, err := nodeAddresses(&droplet)
			if err != nil {
				klog.Errorf("Error getting node addresses for %s: %s", droplet.Name, err)
				continue
			}
			for _, address := range addresses {
				if missingDroplets[address.Address] {
					dropletIDs = append(dropletIDs, droplet.ID)
					delete(missingDroplets, droplet.Name)
					break
				}
			}
		}
	}

	if len(missingDroplets) > 0 {
		// Sort node names for stable output.
		missingNames := make([]string, 0, len(missingDroplets))
		for missingName := range missingDroplets {
			missingNames = append(missingNames, missingName)
		}
		sort.Strings(missingNames)

		klog.Errorf("Failed to find droplets for nodes %s", strings.Join(missingNames, " "))
	}

	return dropletIDs, nil
}

func buildLoadBalancerRequest(ctx context.Context, service *v1.Service, godoClient *godo.Client) (*godo.LoadBalancerRequest, error) {
	lbName := getLoadBalancerName(service)

	lbType, err := getType(service)
	if err != nil {
		return nil, err
	}
	lbNetwork, err := getNetwork(service)
	if err != nil {
		return nil, err
	}
	var forwardingRules []godo.ForwardingRule
	if lbType == godo.LoadBalancerTypeRegionalNetwork {
		forwardingRules, err = buildRegionalNetworkForwardingRule(service)
		if err != nil {
			return nil, err
		}
	} else {
		forwardingRules, err = buildForwardingRules(ctx, service, godoClient)
		if err != nil {
			return nil, err
		}
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

	sizeSlug, err := getSizeSlug(service)
	if err != nil {
		return nil, err
	}

	sizeUnit, err := getSizeUnit(service)
	if err != nil {
		return nil, err
	}

	if sizeSlug != "" && sizeUnit > 0 {
		return nil, fmt.Errorf("only one of LB size slug and size unit can be provided")
	}

	redirectHTTPToHTTPS, err := getRedirectHTTPToHTTPS(service)
	if err != nil {
		return nil, err
	}

	enableProxyProtocol, err := getEnableProxyProtocol(service)
	if err != nil {
		return nil, err
	}

	enableBackendKeepalive, err := getEnableBackendKeepalive(service)
	if err != nil {
		return nil, err
	}

	disableLetsEncryptDNSRecords, err := getDisableLetsEncryptDNSRecords(service)
	if err != nil {
		return nil, err
	}

	httpIdleTimeoutSeconds, err := getHttpIdleTimeoutSeconds(service)
	if err != nil {
		return nil, err
	}

	fw, err := buildFirewall(service)
	if err != nil {
		return nil, err
	}

	return &godo.LoadBalancerRequest{
		Name:                         lbName,
		SizeSlug:                     sizeSlug,
		SizeUnit:                     sizeUnit,
		ForwardingRules:              forwardingRules,
		HealthCheck:                  healthCheck,
		StickySessions:               stickySessions,
		Algorithm:                    algorithm,
		RedirectHttpToHttps:          redirectHTTPToHTTPS,
		EnableProxyProtocol:          enableProxyProtocol,
		EnableBackendKeepalive:       enableBackendKeepalive,
		DisableLetsEncryptDNSRecords: &disableLetsEncryptDNSRecords,
		HTTPIdleTimeoutSeconds:       httpIdleTimeoutSeconds,
		Firewall:                     fw,
		Type:                         lbType,
		Network:                      lbNetwork,
	}, nil
}

// buildLoadBalancerRequest returns a *godo.LoadBalancerRequest to balance
// requests for service across nodes.
func (l *loadBalancers) buildLoadBalancerRequest(ctx context.Context, service *v1.Service, nodes []*v1.Node) (*godo.LoadBalancerRequest, error) {
	req, err := buildLoadBalancerRequest(ctx, service, l.resources.gclient)
	if err != nil {
		return nil, err
	}

	dropletIDs, err := l.nodesToDropletIDs(ctx, nodes)
	if err != nil {
		return nil, err
	}
	req.DropletIDs = dropletIDs

	var tags []string
	if l.resources.clusterID != "" {
		tags = []string{buildK8sTag(l.resources.clusterID)}
	}
	req.Tags = tags

	req.Region = l.region
	req.VPCUUID = l.resources.clusterVPCID
	return req, nil
}

// buildHealthChecks returns a godo.HealthCheck for service.
func buildHealthCheck(service *v1.Service) (*godo.HealthCheck, error) {
	// Default health check behavior
	hcPath, hcPort := healthCheckPathAndPort(service)
	hcProtocol := protocolHTTP
	// Disable PROXY protocol when health checking kube-proxy or the health check node port (which don't support it)
	proxyProtocol := godo.PtrTo(false)

	// Permit changing the default health check behavior if the override annotation
	// is set.
	_, ok := service.Annotations[annDOOverrideHealthCheck]
	if ok {
		var err error
		hcPath = healthCheckPath(service)
		hcPort, err = healthCheckPort(service)
		if err != nil {
			return nil, err
		}
		hcProtocol, err = healthCheckProtocol(service)
		if err != nil {
			return nil, err
		}
		// Revert to the behaviour on the LB
		proxyProtocol = nil
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

	return &godo.HealthCheck{
		Protocol:               hcProtocol,
		Port:                   hcPort,
		Path:                   hcPath,
		CheckIntervalSeconds:   checkIntervalSecs,
		ResponseTimeoutSeconds: responseTimeoutSecs,
		UnhealthyThreshold:     unhealthyThreshold,
		HealthyThreshold:       healthyThreshold,
		ProxyProtocol:          proxyProtocol,
	}, nil
}

func buildHTTP3ForwardingRule(ctx context.Context, service *v1.Service, godoClient *godo.Client) (*godo.ForwardingRule, error) {
	http3Port, err := getHTTP3Port(service)
	if err != nil {
		return nil, err
	}
	if http3Port == -1 {
		return nil, nil
	}

	certificateID, err := findCertificateID(ctx, service, godoClient)
	if err != nil {
		return nil, err
	}
	if getTLSPassThrough(service) {
		return nil, errors.New("TLS passthrough is not allowed to be used in conjunction with HTTP3")
	}
	if certificateID == "" {
		return nil, errors.New("certificate ID is required for HTTP3")
	}

	for _, port := range service.Spec.Ports {
		if port.Port == int32(http3Port) {
			return &godo.ForwardingRule{
				EntryProtocol:  protocolHTTP3,
				EntryPort:      http3Port,
				CertificateID:  certificateID,
				TargetProtocol: protocolHTTP,
				TargetPort:     int(port.NodePort),
			}, nil
		}
	}

	return nil, nil
}

// buildForwardingRules returns the forwarding rules of the Load Balancer of
// service.
func buildForwardingRules(ctx context.Context, service *v1.Service, godoClient *godo.Client) ([]godo.ForwardingRule, error) {
	var forwardingRules []godo.ForwardingRule

	defaultProtocol, err := getProtocol(service)
	if err != nil {
		return nil, err
	}

	httpPorts, err := getHTTPPorts(service)
	if err != nil {
		return nil, err
	}

	httpsPorts, err := getHTTPSPorts(service)
	if err != nil {
		return nil, err
	}

	http2Ports, err := getHTTP2Ports(service)
	if err != nil {
		return nil, err
	}

	// NOTE: HTTP3 ports are intentionally not included because a shared port with HTTPS or HTTP2 is required
	portDups := findDups(httpPorts, httpsPorts, http2Ports)
	if len(portDups) > 0 {
		return nil, fmt.Errorf("ports from annotations \"%s*-ports\" cannot be shared but found: %s", annDOLoadBalancerBase, strings.Join(portDups, ", "))
	}

	certificateID, err := findCertificateID(ctx, service, godoClient)
	if err != nil {
		return nil, err
	}
	tlsPassThrough := getTLSPassThrough(service)
	needSecureProto := certificateID != "" || tlsPassThrough

	if needSecureProto && len(httpsPorts) == 0 && !contains(http2Ports, defaultSecurePort) {
		httpsPorts = append(httpsPorts, defaultSecurePort)
	}

	httpPortMap := map[int32]bool{}
	for _, port := range httpPorts {
		httpPortMap[int32(port)] = true
	}
	httpsPortMap := map[int32]bool{}
	for _, port := range httpsPorts {
		httpsPortMap[int32(port)] = true
	}
	http2PortMap := map[int32]bool{}
	for _, port := range http2Ports {
		http2PortMap[int32(port)] = true
	}

	for _, port := range service.Spec.Ports {
		protocol := defaultProtocol
		if httpPortMap[port.Port] {
			protocol = protocolHTTP
		}
		if httpsPortMap[port.Port] {
			protocol = protocolHTTPS
		}
		if http2PortMap[port.Port] {
			protocol = protocolHTTP2
		}

		if port.Protocol == v1.ProtocolUDP {
			protocol = protocolUDP
		}

		forwardingRule, err := buildForwardingRule(service, &port, protocol, certificateID, tlsPassThrough)
		if err != nil {
			return nil, err
		}
		forwardingRules = append(forwardingRules, *forwardingRule)
	}

	if h3, err := buildHTTP3ForwardingRule(ctx, service, godoClient); err != nil {
		return nil, fmt.Errorf("failed to construct http3 forwarding rule: %w", err)
	} else if h3 != nil {
		forwardingRules = append(forwardingRules, *h3)
	}

	return forwardingRules, nil
}

func buildRegionalNetworkForwardingRule(service *v1.Service) ([]godo.ForwardingRule, error) {
	var forwardingRules []godo.ForwardingRule
	for _, port := range service.Spec.Ports {
		var protocol string
		switch port.Protocol {
		case portProtocolTCP:
			protocol = protocolTCP
		case portProtocolUDP:
			protocol = protocolUDP
		default:
			return nil, fmt.Errorf("only TCP or UDP protocol is supported, got: %q", port.Protocol)
		}
		forwardingRules = append(forwardingRules, godo.ForwardingRule{
			EntryProtocol:  protocol,
			EntryPort:      int(port.Port),
			TargetProtocol: protocol,
			TargetPort:     int(port.Port),
		})
	}
	return forwardingRules, nil
}

func buildForwardingRule(service *v1.Service, port *v1.ServicePort, protocol, certificateID string, tlsPassThrough bool) (*godo.ForwardingRule, error) {
	var forwardingRule godo.ForwardingRule

	switch port.Protocol {
	case portProtocolTCP, portProtocolUDP:
	default:
		return nil, fmt.Errorf("only TCP or UDP protocol is supported, got: %q", port.Protocol)
	}

	forwardingRule.EntryProtocol = protocol
	forwardingRule.TargetProtocol = protocol

	forwardingRule.EntryPort = int(port.Port)
	forwardingRule.TargetPort = int(port.NodePort)

	if protocol == protocolHTTPS || protocol == protocolHTTP2 {
		err := buildTLSForwardingRule(&forwardingRule, service, port.Port, certificateID, tlsPassThrough)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS part(s) of forwarding rule: %s", err)
		}
	}

	return &forwardingRule, nil
}

func buildFirewall(service *v1.Service) (*godo.LBFirewall, error) {
	var allow []string
	sourceRanges, err := getSourceRangeRules(service)
	if err != nil {
		return nil, fmt.Errorf("failed to get source range rules: %s", err)
	}
	if len(sourceRanges) > 0 {
		// source ranges should take precedence over annotation based allow rules
		allow = sourceRanges
	} else {
		allow = getStrings(service, annDOAllowRules)
	}
	return &godo.LBFirewall{
		Deny:  getStrings(service, annDODenyRules),
		Allow: allow,
	}, nil
}

// getSourceRangeRules returns loadBalancerSourceRanges in the format
// `cidr:{source}`
func getSourceRangeRules(service *v1.Service) ([]string, error) {
	if len(service.Spec.LoadBalancerSourceRanges) == 0 {
		return nil, nil
	}
	specs := service.Spec.LoadBalancerSourceRanges
	ipnets, err := utilnet.ParseIPNets(specs...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse spec.loadBalancerSourceRanges %q: %s (expected format for an IP address is 10.0.0.0/24)", specs, err)
	}

	var sourceAllows []string
	for _, sourceRange := range ipnets {
		sourceAllows = append(sourceAllows, fmt.Sprintf("cidr:%s", sourceRange))
	}
	return sourceAllows, nil
}

func buildTLSForwardingRule(forwardingRule *godo.ForwardingRule, service *v1.Service, port int32, certificateID string, tlsPassThrough bool) error {
	if certificateID == "" && !tlsPassThrough {
		return errors.New("must set certificate id or enable tls pass through")
	}

	if certificateID != "" && tlsPassThrough {
		return errors.New("either certificate id should be set or tls pass through enabled, not both")
	}

	if tlsPassThrough {
		forwardingRule.TlsPassthrough = tlsPassThrough
		// We don't explicitly set the TargetProtocol here since in buildForwardingRule
		// we already assign the annotation-defined protocol to both the EntryProtocol
		// and TargetProtocol, and in the tlsPassthrough case we want the TargetProtocol
		// to match the EntryProtocol.
	} else {
		forwardingRule.CertificateID = certificateID
		forwardingRule.TargetProtocol = protocolHTTP
	}

	return nil
}

func buildStickySessions(service *v1.Service) (*godo.StickySessions, error) {
	t := getStickySessionsType(service)

	if t == stickySessionsTypeNone {
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
		return protocolTCP, nil
	}

	switch protocol {
	case protocolTCP, protocolHTTP, protocolHTTPS, protocolHTTP2, protocolHTTP3:
	default:
		return "", fmt.Errorf("invalid protocol %q specified in annotation %q", protocol, annDOProtocol)
	}

	return protocol, nil
}

// getHostname returns the desired hostname for the LB service.
func getHostname(service *v1.Service) string {
	return strings.ToLower(service.Annotations[annDOHostname])
}

// healthCheckPort returns the health check port specified, defaulting
// to the first port in the service otherwise.
func healthCheckPort(service *v1.Service) (int, error) {
	ports, err := getPorts(service, annDOHealthCheckPort)
	if err != nil {
		return 0, fmt.Errorf("failed to get health check port: %v", err)
	}

	if len(ports) > 1 {
		return 0, fmt.Errorf("annotation %s only supports a single port, but found multiple: %v", annDOHealthCheckPort, ports)
	}

	if len(ports) == 1 {
		for _, servicePort := range service.Spec.Ports {
			if int(servicePort.Port) == ports[0] {
				if servicePort.Protocol == v1.ProtocolUDP {
					return 0, fmt.Errorf("health check port: %d, cannot be of protocol type UDP", servicePort.Port)
				}
				return int(servicePort.NodePort), nil
			}
		}
		return 0, fmt.Errorf("specified health check port %d does not exist on service %s/%s", ports[0], service.Namespace, service.Name)
	}

	// return the first TCP port found as the healthcheck port
	for _, p := range service.Spec.Ports {
		if p.Protocol == v1.ProtocolTCP {
			return int(p.NodePort), nil
		}
	}

	// Otherwise no TCP ports were found
	return 0, errors.New("no health check port of protocol TCP found")
}

// healthCheckProtocol returns the health check protocol as specified in the service,
// falling back to TCP if not specified.
func healthCheckProtocol(service *v1.Service) (string, error) {
	protocol := service.Annotations[annDOHealthCheckProtocol]
	path := healthCheckPath(service)

	if protocol == "" {
		if path != "" {
			return protocolHTTP, nil
		}
		return protocolTCP, nil
	}

	switch protocol {
	case protocolTCP, protocolHTTP, protocolHTTPS:
	default:
		return "", fmt.Errorf("invalid protocol %q specified in annotation %q", protocol, annDOHealthCheckProtocol)
	}

	return protocol, nil
}

// healthCheckPathAndPort returns the health check path and port
func healthCheckPathAndPort(service *v1.Service) (string, int) {
	// If a health check node port is allocated, use that to health check
	path, healthCheckNodePort := servicehelpers.GetServiceHealthCheckPathPort(service)
	if path != "" {
		return path, int(healthCheckNodePort)
	}
	// Otherwise health check kube-proxy
	return "/healthz", kubeProxyHealthPort
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

// getHTTPPorts returns the ports for the given service that are set to use
// HTTP.
func getHTTPPorts(service *v1.Service) ([]int, error) {
	return getPorts(service, annDOHTTPPorts)
}

// getHTTP2Ports returns the ports for the given service that are set to use
// HTTP2.
func getHTTP2Ports(service *v1.Service) ([]int, error) {
	return getPorts(service, annDOHTTP2Ports)
}

// getHTTP3Port returns the port for the given service that is set to use HTTP3 as the entry protocol.
func getHTTP3Port(service *v1.Service) (int, error) {
	portStr, ok := service.Annotations[annDOHTTP3Port]
	if !ok {
		return -1, nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return -1, err
	}

	return port, nil
}

// getHTTPSPorts returns the ports for the given service that are set to use
// HTTPS.
func getHTTPSPorts(service *v1.Service) ([]int, error) {
	return getPorts(service, annDOTLSPorts)
}

// getPorts returns the ports for the given service and annotation.
func getPorts(service *v1.Service, anno string) ([]int, error) {
	ports, ok := service.Annotations[anno]
	if !ok {
		return nil, nil
	}

	portsSlice := strings.Split(ports, ",")

	portsInt := make([]int, len(portsSlice))
	for i, port := range portsSlice {
		port, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}

		portsInt[i] = port
	}

	return portsInt, nil
}

// getStrings returns the strings for the given service and annotation.
func getStrings(service *v1.Service, anno string) []string {
	vals, ok := service.Annotations[anno]
	if !ok || len(vals) == 0 {
		return nil
	}

	pieces := strings.Split(vals, ",")
	for i := range pieces {
		pieces[i] = strings.TrimSpace(pieces[i])
	}
	return pieces
}

// getCertificateID returns the certificate ID of service to use for forwarding
// rules.
func getCertificateID(service *v1.Service) string {
	return service.Annotations[annDOCertificateID]
}

// getCertificateName returns the certificate name of service to use for forwarding
// rules.
func getCertificateName(service *v1.Service) string {
	return service.Annotations[annDOCertificateName]
}

func findCertificateID(ctx context.Context, service *v1.Service, godoClient *godo.Client) (string, error) {
	certificateID := getCertificateID(service)
	if certificateID != "" {
		return certificateID, nil
	}
	certificateName := getCertificateName(service)
	if certificateName == "" {
		return "", nil
	}
	lbCert, _, err := godoClient.Certificates.ListByName(ctx, certificateName, &godo.ListOptions{Page: 1, PerPage: 1})
	if err != nil {
		return "", fmt.Errorf("failed to get certificate by name: %q error: %s", certificateName, err)
	}
	if len(lbCert) == 0 {
		return "", fmt.Errorf("certificate with name: %q not found", certificateName)
	}
	return lbCert[0].ID, nil
}

// getTLSPassThrough returns true if there should be TLS pass through to
// backend nodes.
func getTLSPassThrough(service *v1.Service) bool {
	passThrough, _, err := getBool(service.Annotations, annDOTLSPassThrough)
	return err == nil && passThrough
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

// getSizeSlug returns the load balancer size as a slug
func getSizeSlug(service *v1.Service) (string, error) {
	sizeSlug := service.Annotations[annDOSizeSlug]

	if sizeSlug != "" {
		switch sizeSlug {
		case "lb-small", "lb-medium", "lb-large":
		default:
			return "", fmt.Errorf("invalid LB size slug provided: %s", sizeSlug)
		}
	}

	return sizeSlug, nil
}

// getSizeUnit returns the load balancer size as a number
func getSizeUnit(service *v1.Service) (uint32, error) {
	sizeUnitStr, ok := service.Annotations[annDOSizeUnit]

	if !ok || sizeUnitStr == "" {
		return uint32(0), nil
	}

	sizeUnit, err := strconv.Atoi(sizeUnitStr)
	if err != nil {
		return 0, fmt.Errorf("invalid LB size unit %q provided: %s", sizeUnitStr, err)
	}

	if sizeUnit < 0 {
		return 0, fmt.Errorf("LB size unit must be non-negative. %d provided", sizeUnit)
	}

	return uint32(sizeUnit), nil
}

// getStickySessionsType returns the sticky session type to use for
// loadbalancer. none is returned when a type is not specified.
func getStickySessionsType(service *v1.Service) string {
	t := service.Annotations[annDOStickySessionsType]

	switch t {
	case stickySessionsTypeCookies:
		return stickySessionsTypeCookies
	default:
		return stickySessionsTypeNone
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
func getRedirectHTTPToHTTPS(service *v1.Service) (bool, error) {
	redirectHTTPToHTTPS, _, err := getBool(service.Annotations, annDORedirectHTTPToHTTPS)
	if err != nil {
		return false, fmt.Errorf("failed to get HTTP-to-HTTPS configuration setting: %s", err)
	}
	return redirectHTTPToHTTPS, nil
}

// getEnableProxyProtocol returns whether PROXY protocol should be enabled.
// False is returned if not specified.
func getEnableProxyProtocol(service *v1.Service) (bool, error) {
	enableProxyProtocol, _, err := getBool(service.Annotations, annDOEnableProxyProtocol)
	if err != nil {
		return false, fmt.Errorf("failed to get proxy protocol configuration setting: %s", err)
	}
	return enableProxyProtocol, nil
}

// getDisableLetsEncryptDNSRecords returns whether DNS records should be automatically created
// for Let's Encrypt certs added to the LB
func getDisableLetsEncryptDNSRecords(service *v1.Service) (bool, error) {
	disableLetsEncryptDNSRecords, _, err := getBool(service.Annotations, annDODisableLetsEncryptDNSRecords)
	if err != nil {
		return false, fmt.Errorf("failed to get disable lets encrypt dns records configuration setting: %s", err)
	}

	return disableLetsEncryptDNSRecords, nil
}

// getEnableBackendKeepalive returns whether HTTP keepalive to target droplets should be enabled.
// False is returned if not specified.
func getEnableBackendKeepalive(service *v1.Service) (bool, error) {
	enableBackendKeepalive, _, err := getBool(service.Annotations, annDOEnableBackendKeepalive)
	if err != nil {
		return false, fmt.Errorf("failed to get backend keepalive configuration setting: %s", err)
	}
	return enableBackendKeepalive, nil
}

func getHttpIdleTimeoutSeconds(service *v1.Service) (*uint64, error) {
	httpIdleTimeoutSeconds, ok := service.Annotations[annDOHttpIdleTimeoutSeconds]
	if !ok {
		return nil, nil
	}

	httpIdleTimeout, err := strconv.ParseUint(httpIdleTimeoutSeconds, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse provided LB HTTP idle timeout seconds value '%s': %s", httpIdleTimeoutSeconds, err)
	}

	return &httpIdleTimeout, nil
}

func getLoadBalancerID(service *v1.Service) string {
	return service.ObjectMeta.Annotations[annDOLoadBalancerID]
}

// getDisownLB returns whether the load-balancer should be disowned.
// False is returned if not specified.
func getDisownLB(service *v1.Service) (bool, error) {
	disownLB, _, err := getBool(service.Annotations, annDODisownLB)
	if err != nil {
		return false, fmt.Errorf("failed to get disown LB configuration setting: %s", err)
	}
	return disownLB, nil
}

func getType(service *v1.Service) (string, error) {
	name, ok := service.Annotations[annDOType]
	if !ok || name == "" {
		return defaultLBType, nil
	}
	if !(name == godo.LoadBalancerTypeRegional || name == godo.LoadBalancerTypeRegionalNetwork) {
		return "", fmt.Errorf("only LB types supported are (%s, %s)", godo.LoadBalancerTypeRegional, godo.LoadBalancerTypeRegionalNetwork)
	}
	return name, nil
}

func getNetwork(service *v1.Service) (string, error) {
	network := service.Annotations[annDONetwork]
	if network == "" {
		return godo.LoadBalancerNetworkTypeExternal, nil
	}
	if !(network == godo.LoadBalancerNetworkTypeExternal || network == godo.LoadBalancerNetworkTypeInternal) {
		return "", fmt.Errorf("only LB networks supported are (%s, %s)", godo.LoadBalancerNetworkTypeExternal, godo.LoadBalancerNetworkTypeInternal)
	}
	return network, nil
}

func findDups(lists ...[]int) []string {
	occurrences := map[int]int{}

	for _, list := range lists {
		for _, val := range list {
			occurrences[val]++
		}
	}

	var dups []string
	for val, occur := range occurrences {
		if occur > 1 {
			dups = append(dups, strconv.Itoa(val))
		}
	}

	sort.Strings(dups)
	return dups
}

func contains(vals []int, val int) bool {
	for _, v := range vals {
		if v == val {
			return true
		}
	}
	return false
}

// logLBInfo wraps around klog and logs LB operation type and LB configuration info.
func logLBInfo(opType string, cfgInfo *godo.LoadBalancerRequest, logLevel klog.Level) {
	if cfgInfo != nil {
		klog.V(logLevel).Infof("Operation type: %v, Configuration info: %v", opType, cfgInfo)
	}
}
