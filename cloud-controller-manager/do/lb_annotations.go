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

const (
	// annDOLoadBalancerID is the annotation specifying the load-balancer ID
	// used to enable fast retrievals of load-balancers from the API by UUID.
	annDOLoadBalancerID = "kubernetes.digitalocean.com/load-balancer-id"

	annDOLoadBalancerBase = "service.beta.kubernetes.io/do-loadbalancer-"

	// annDOLoadBalancerName is the annotation used to specify a custom name
	// for the load balancer.
	annDOLoadBalancerName = annDOLoadBalancerBase + "name"

	// annDOProtocol is the annotation used to specify the default protocol
	// for DO load balancers. For ports specified in annDOTLSPorts, this protocol
	// is overwritten to https. Options are tcp, http and https. Defaults to tcp.
	annDOProtocol = annDOLoadBalancerBase + "protocol"

	// annDOHealthCheckPath is the annotation used to specify the health check path
	// for DO load balancers. Defaults to '/'.
	annDOHealthCheckPath = annDOLoadBalancerBase + "healthcheck-path"

	// annDOHealthCheckPort is the annotation used to specify the health check port
	// for DO load balancers. Defaults to the first Service Port
	annDOHealthCheckPort = annDOLoadBalancerBase + "healthcheck-port"

	// annDOHealthCheckProtocol is the annotation used to specify the health check protocol
	// for DO load balancers. Defaults to the protocol used in annDOProtocol.
	annDOHealthCheckProtocol = annDOLoadBalancerBase + "healthcheck-protocol"

	// annDOHealthCheckIntervalSeconds is the annotation used to specify the
	// number of seconds between between two consecutive health checks. The
	// value must be between 3 and 300. Defaults to 3.
	annDOHealthCheckIntervalSeconds = annDOLoadBalancerBase + "healthcheck-check-interval-seconds"

	// annDOHealthCheckResponseTimeoutSeconds is the annotation used to specify the
	// number of seconds the Load Balancer instance will wait for a response
	// until marking a health check as failed. The value must be between 3 and
	// 300. Defaults to 5.
	annDOHealthCheckResponseTimeoutSeconds = annDOLoadBalancerBase + "healthcheck-response-timeout-seconds"

	// annDOHealthCheckUnhealthyThreshold is the annotation used to specify the
	// number of times a health check must fail for a backend Droplet to be
	// marked "unhealthy" and be removed from the pool for the given service.
	// The value must be between 2 and 10. Defaults to 3.
	annDOHealthCheckUnhealthyThreshold = annDOLoadBalancerBase + "healthcheck-unhealthy-threshold"

	// annDOHealthCheckHealthyThreshold is the annotation used to specify the
	// number of times a health check must pass for a backend Droplet to be
	// marked "healthy" for the given service and be re-added to the pool. The
	// value must be between 2 and 10. Defaults to 5.
	annDOHealthCheckHealthyThreshold = annDOLoadBalancerBase + "healthcheck-healthy-threshold"

	// annDOHTTPPorts is the annotation used to specify which ports of the load balancer
	// should use the HTTP protocol. This is a comma separated list of ports
	// (e.g., 80,8080).
	annDOHTTPPorts = annDOLoadBalancerBase + "http-ports"

	// annDOTLSPorts is the annotation used to specify which ports of the load balancer
	// should use the HTTPS protocol. This is a comma separated list of ports
	// (e.g., 443,6443,7443).
	annDOTLSPorts = annDOLoadBalancerBase + "tls-ports"

	// annDOHTTP2Ports is the annotation used to specify which ports of the load balancer
	// should use the HTTP2 protocol. This is a comma separated list of ports
	// (e.g., 443,6443,7443).
	annDOHTTP2Ports = annDOLoadBalancerBase + "http2-ports"

	// annDOHTTP3Port is the annotation used to specify which port of the load balancer
	// should use the HTTP3 protocol. Unlike the other annotations, this is for a single
	// port, as the Load Balancer configuration only allows one HTTP3 forwarding rule.
	annDOHTTP3Port = annDOLoadBalancerBase + "http3-port"

	// annDOTLSPassThrough is the annotation used to specify whether the
	// DO loadbalancer should pass encrypted data to backend droplets.
	// This is optional and defaults to false.
	annDOTLSPassThrough = annDOLoadBalancerBase + "tls-passthrough"

	// annDOCertificateID is the annotation specifying the certificate ID
	// used for https protocol. This annotation is required if annDOTLSPorts
	// is passed.
	annDOCertificateID = annDOLoadBalancerBase + "certificate-id"

	// annDOHostname is the annotation specifying the hostname to use for the LB.
	annDOHostname = annDOLoadBalancerBase + "hostname"

	// annDOAlgorithm is the annotation specifying which algorithm DO load balancer
	// should use. Options are round_robin and least_connections. Defaults
	// to round_robin.
	// DEPRECATED: Specifying this annotation does nothing, as the algorithm field
	// on the API is ignored.
	annDOAlgorithm = annDOLoadBalancerBase + "algorithm"

	// annDOSizeSlug is the annotation specifying the size of the LB.
	// Options are `lb-small`, `lb-medium`, and `lb-large`.
	// Defaults to `lb-small`. Only one of annDOSizeSlug and annDOSizeUnit can be specified.
	annDOSizeSlug = annDOLoadBalancerBase + "size-slug"

	// annDOSizeUnit is the annotation specifying the size of the LB.
	// Options are numbers greater than or equal to `1`. Only one of annDOSizeUnit and annDOSizeSlug can be specified.
	annDOSizeUnit = annDOLoadBalancerBase + "size-unit"

	// annDOStickySessionsType is the annotation specifying which sticky session type
	// DO loadbalancer should use. Options are none and cookies. Defaults
	// to none.
	annDOStickySessionsType = annDOLoadBalancerBase + "sticky-sessions-type"

	// annDOStickySessionsCookieName is the annotation specifying what cookie name to use for
	// DO loadbalancer sticky session. This annotation is required if
	// annDOStickySessionType is set to cookies.
	annDOStickySessionsCookieName = annDOLoadBalancerBase + "sticky-sessions-cookie-name"

	// annDOStickySessionsCookieTTL is the annotation specifying TTL of cookie used for
	// DO load balancer sticky session. This annotation is required if
	// annDOStickySessionType is set to cookies.
	annDOStickySessionsCookieTTL = annDOLoadBalancerBase + "sticky-sessions-cookie-ttl"

	// annDORedirectHTTPToHTTPS is the annotation specifying whether or not Http traffic
	// should be redirected to Https. Defaults to false
	annDORedirectHTTPToHTTPS = annDOLoadBalancerBase + "redirect-http-to-https"

	// annDODisableLetsEncryptDNSRecords is the annotation specifying whether automatic DNS record creation should
	// be disabled when a Let's Encrypt cert is added to a load balancer
	annDODisableLetsEncryptDNSRecords = annDOLoadBalancerBase + "disable-lets-encrypt-dns-records"

	// annDOEnableProxyProtocol is the annotation specifying whether PROXY protocol should
	// be enabled. Defaults to false.
	annDOEnableProxyProtocol = annDOLoadBalancerBase + "enable-proxy-protocol"

	// annDOEnableBackendKeepalive is the annotation specifying whether HTTP keepalive connections
	// should be enabled to backend target droplets. Defaults to false.
	annDOEnableBackendKeepalive = annDOLoadBalancerBase + "enable-backend-keepalive"

	// annDODisownLB is the annotation specifying if a load-balancer should be
	// disowned. Defaults to false.
	annDODisownLB = "service.kubernetes.io/do-loadbalancer-disown"

	// annDOHttpIdleTimeoutSeconds is the annotation for specifying the http idle timeout configuration in seconds
	// this defaults to 60
	annDOHttpIdleTimeoutSeconds = annDOLoadBalancerBase + "http-idle-timeout-seconds"

	// annDODenyRules is the annotation used to specify DENY rules for the load-balancer's firewall
	// This is a comma separated list of rules, rules must be in the format "{type}:{source}"
	// e.g. - ip:1.2.3.4,cidr:2.3.0.0/16
	annDODenyRules = annDOLoadBalancerBase + "deny-rules"

	// annDOAllowRules is the annotation used to specify ALLOW rules for the load-balancer's firewall
	// This is a comma separated list of rules, rules must be in the format "{type}:{source}"
	// e.g. - ip:1.2.3.4,cidr:2.3.0.0/16
	annDOAllowRules = annDOLoadBalancerBase + "allow-rules"

	// annDOType is the annotation used to specify the type of the load balancer. Either REGIONAL or REGIONAL_NETWORK (currently in closed alpha)
	// are permitted. If no type is provided, then it will default REGIONAL.
	annDOType = annDOLoadBalancerBase + "type"
)
