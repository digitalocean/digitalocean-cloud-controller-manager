## unreleased

## v0.1.58 (beta) - January 17, 2025

* Add IPv6 address to the LoadBalancer Service Ingress list (@jvasilevsky)
* Update dependencies: (@eomolo)
  - k8s.io packages v0.32.0
  - sigs.k8s.io/controller-runtime v0.20.0
  - github.com/prometheus/client_golang v1.20.5
  - godo v1.133.0
* Update go to 1.23.4 (@eomolo)
* Reduce the number of results per page when listing objects via DO API (@d-honeybadger)

## v0.1.57 (beta) - December 3, 2024

* Update dependencies
  - godo v1.131.0
* Migrate to CCM InstancesV2

## v0.1.56 (beta) - August 26, 2024

* Update dependencies: (@d-honeybadger)
  - k8s.io packages v0.31.0
  - sigs.k8s.io/controller-runtime v0.19.0
  - github.com/prometheus/client_golang v1.20.2
* Update go to 1.23 (@d-honeybadger)

## v0.1.55 (beta) - July 29, 2024

* When using the LoadBalancer `service.beta.kubernetes.io/do-loadbalancer-type=REGIONAL_NETWORK` (under closed beta), firewall rules
are added to open up the underlying health check port and all the defined (port, protocols) defined on the service. This is to
permit traffic to arrive directly on the underlying worker nodes.

## v0.1.54 (beta) - June 12, 2024

* Fixes an issue with load balancer health checks when the LB is using PROXY protocol. The new health check 
  implementation (introduced in v0.1.51), now probes either kube proxy (Cluster) or the health check node port (Local).
  If the LB enables PROXY protocol, this alters the health check behavior to also use PROXY protocol. Since these Kubernetes
  components don't support PROXY protocol, this caused worker nodes to be marked as unhealthy. Support was added to the
  load balancer health check to optionally enable/disable PROXY protocol. When using the default health check implementation
  the health check will disable PROXY protocol. If the `service.beta.kubernetes.io/do-loadbalancer-override-health-check` is
  provided, then the health check will default to the setting on the LB.

## v0.1.53 (beta) - June  7, 2024

* Adding support for internal load balancers (NOTE: this is a closed beta feature, contact DigitalOcean 
  support to review its public availability). This is enabled by setting a new annotation `service.beta.kubernetes.io/do-loadbalancer-network`
  to `INTERNAL`.

## v0.1.52 (beta) - June  4, 2024

* Adding support for specifiying `loadBalancerSourceRanges` in the service spec. Source ranges take precedence over annotation based allow rules (`service.beta.kubernetes.io/do-loadbalancer-allow-rules`).

## v0.1.51 (beta) - May 28, 2024

* Adjusts load balancer health check behaviour to probe Kubernetes components correctly, ensuring that LB traffic stops
  in time in case of unavailability and pending node replacements. The concrete health check configuration depends on
  the specified external traffic policy. See [the extended documentation](docs/getting-started.md#health-check-configuration) for details.
* Adding new annotation `service.beta.kubernetes.io/do-loadbalancer-certificate-name` to configure which TLS certificate
  to use for HTTPs forwarding rules. This can be used instead of `service.beta.kubernetes.io/do-loadbalancer-certificate-id` which
  needs to be manually updated when using Let's Encrypt certificates. This is due to the certificate ID updating each time the
  certificate is rotated. If both annotations are provided, then `service.beta.kubernetes.io/do-loadbalancer-certificate-id` takes
  precedence.

## v0.1.50 (beta) - May 7, 2024
* Updates kubernetes dependencies: (@ihwang)
  - k8s.io/api@v0.30.0
  - k8s.io/apimachinery@v0.30.0
  - k8s.io/client-go@v0.30.0
  - k8s.io/cloud-provider@v0.30.0
  - k8s.io/component-base@v0.30.0
* Update Go to v1.22 (@ihwang)

## v0.1.49 (beta) - April 11, 2024
* Fix load balancer admission webhook to handle load balancer ID changes gracefully (@prichrd)

## v0.1.48 (beta) - February  6, 2024
* Split release manifests to allow optional deployment of admission server (@prichrd)
* Add documentation for the admission server (@prichrd)

## v0.1.47 (beta) - February  1, 2024
* Add validation webhook for LoadBalancer services (@prichrd)

## v0.1.46 (beta) - January  9, 2024
* Updates kubernetes dependencies: (@kperath)
  - k8s.io/api@v0.29.0
  - k8s.io/apimachinery@v0.29.0
  - k8s.io/client-go@v0.29.0
  - k8s.io/cloud-provider@v0.29.0
  - k8s.io/component-base@v0.29.0
* Updates K8s Registry Location (@userbradley)

## v0.1.45 (beta) - August 22, 2023
* Update retry duration when DO LoadBalancer is being created (@olove) 
* Updates kubernetes dependencies: (@olove)
  - k8s.io/api@v0.28.0
  - k8s.io/apimachinery@v0.28.0
  - k8s.io/client-go@v0.28.0
  - k8s.io/cloud-provider@v0.28.0
  - k8s.io/component-base@v0.28.0

## v0.1.44 (beta) - August  2, 2023
* Remove deny and allow load balancer firewall rules when annotations are removed (@olove) 

## v0.1.43 (beta) - May 25, 2023
* Updates kubernetes dependencies: (@olove)
  - k8s.io/api@v0.27.2
  - k8s.io/apimachinery@v0.27.2
  - k8s.io/client-go@v0.27.2
  - k8s.io/cloud-provider@v0.27.2
  - k8s.io/component-base@v0.27.2

## v0.1.42 (beta) - January  10, 2023
* Updates kubernetes dependencies: (@olove)
  - k8s.io/api@v0.26.0
  - k8s.io/apimachinery@v0.26.0
  - k8s.io/client-go@v0.26.0
  - k8s.io/cloud-provider@v0.26.0
  - k8s.io/component-base@v0.26.0

## v0.1.41 (beta) - January  3, 2023

* Add annotation for customizing Load Balancer HTTP Idle Timeout (@StephenVarela)
* Add annotations for Load Balancers Firewalls (@jrolheiser)
* Relax validation for Load Balancers UDP ports (@anitgandhi)
* Deprecate annotation for customizing Load Balancer algorithm (@anitgandhi)

## v0.1.40 (beta) - November 15, 2022

* Support setting DO API rate limit (@timoreimann)
* Update Go to v1.19 (@timoreimann)
* Support specifying region explicitly (@shatoboar)
* Support custom annotation to specify HTTP3 entry ports for Load Balancers (@anitgandhi)

## v0.1.39 (beta) - August 17, 2022
* Updates kubernetes dependencies:
  - k8s.io/api@v0.24.3
  - k8s.io/apimachinery@v0.24.3
  - k8s.io/client-go@v0.24.3 
  - k8s.io/cloud-provider@v0.24.3
  - k8s.io/component-base@v0.24.3

## v0.1.37 (beta) - April 11, 2022

* add UDP protocol support (@dikshant)
* Bump k8s.io/klog/v2 from 2.9.0 to 2.50.2
* Update godo to v1.78.0 (@cpanato)
* Update Kubernetes dependencies (@cpanato)

## v0.1.36 (beta) - January 14, 2022

* Update Kubernetes dependencies to 1.22.5 (@cshoop)

## v0.1.35 (beta) - Oct 18 2021

* Add annotation for specifying load balancer size unit (@wez470)
* Add annotation for disabling automatic DNS record creation for load balancer Let's Encrypt certs (@wez470)

## v0.1.34 (beta) - Sept 7 2021

* Update Kubernetes dependencies to 1.21.3 (@varshavaradarajan)

## v0.1.33 (beta) - June 24 2021

* Update Kubernetes dependencies to 1.21.2 (@adamwg)

## v0.1.32 (beta) - March 21 2021

* Do not forget work item on firewall controller error (@timoreimann)

## v0.1.31 (beta) - January 30 2021

* Fix broken firewall counter metrics by incrementing (@timoreimann)

## v0.1.30 (beta) - October 31th 2020

* Support LB custom size slug (@anitgandhi)

## v0.1.29 (beta) - October 22th 2020

* Improve firewall metrics design (@timoreimann)
* Support marking Services as firewall-unmanaged (@timoreimann)
* Update Kubernetes dependencies to 1.19.3 (@timoreimann)

## v0.1.28 (beta) - October 15th 2020

* Fix firewall cache usage (@timoreimann)
* Create context after retrieving item from worker queue (@MorrisLaw)
* Fix logging and update Kubernetes dependencies to 1.19.2 (@timoreimann)
* Expose health check failures (@timoreimann)

## v0.1.27 (beta) - September 24th 2020

* Add exponential retry to firewall controller (@MorrisLaw)
* Update Kubernetes dependencies to 1.19.1 (@adamwg)
* Add prometheus metrics instrumentation to firewall controller (@MorrisLaw)
* Add controller to manage worker firewall for public access (@MorrisLaw)
* Support HTTPS as health check protocol (@timoreimann)

## v0.1.26 (beta) - June 16th 2020

* Update Kubernetes dependences to 1.18.3 (@waynr)

## v0.1.25 (beta) - June 15th 2020

* Support disowning LBs (@timoreimann)
* Add commented out leases RBAC rules to manifest (@waynr)

## v0.1.24 (beta) - April 28th 2020

* Add annotation to specify HTTP ports explicitly (@timoreimann)
* Build using Go 1.14 (@timoreimann)
* Add support for enabling backend keepalive feature for load balancers (@anitgandhi)
* Bump godo dependency to v1.35.1 (@anitgandhi)
* Use correct annotation name for invalid health check protocol (@timoreimann)
* Add logging for Create and Update requests to the LB API (@morrislaw)
* Add support for specifying custom load-balancer names (@grzesiek)
* Support specifying a fake region by environment variable (@timoreimann)
* Update Kubernetes dependencies to 1.17.5 (@waynr)

## v0.1.23 (beta) - Jan 31th 2020

### Added

* Add `service.beta.kubernetes.io/do-loadbalancer-healthcheck-port` annotation to customize DO LB health-check port (@ntate)

### Fixed

* Maintain default protocol when secure protocol override is applied (@timoreimann)

## v0.1.22 (beta) - Jan 15th 2020

* Add `DEBUG_ADDR` environment variable for configuring the address of an HTTP server serving a `/healthz` health endpoint (@nanzhong)

## v0.1.21 (beta) - Oct 27th 2019

### Changed

* Update Deployment release manifest API version from removed extensions/v1beta1 to apps/v1 (@timoreimann)
* Update Kubernetes dependencies to 1.16.2 (@timoreimann)

## v0.1.20 (beta) - Sept 9th 2019

### Fixed

* loadbalancers: improve handling of DigitalOcean Let's Encrypt certificates that have been automatically rotated by DigitalOcean's LBaaS (@waynr)

## v0.1.19 (beta) - Aug 28th 2019

### Changed

* Overwrite service load-balancer ID on mismatch (@timoreimann)

## v0.1.18 (beta) - Aug 9th 2019

### Changed

* Reduce API interactions around LB tag synchronization (@timoreimann)

## v0.1.17 (beta) - Aug 6th 2019

### Added

* Support LB with status.Hostname instead of status.IP (@snormore)
* Support custom annotation to specify HTTP2 ports (@timoreimann)
* Use provider ID for setting LB droplet targets (@timoreimann)
* Annotate Service objects by load-balancer UUIDs to enable free LB renames and improve the DO API consumption performance (@timoreimann)

### Fixed

* Do not force HTTP with sticky-sessions (@snormore)
* Set default health check protocol to HTTP if health check path is given (@snormore)

## v0.1.16 (beta) - Jul 16th 2019

### Added

* HTTP/2 support for LB services (@snormore)

### Changed

* Update Kubernetes dependencies to 1.15.0 (@timoreimann)
* Set default LB health check protocol to TCP if not specified (@snormore)
* Default to HTTP for sticky sessions if no protocol is defined (@snormore)

### Fixed

* Do not return error when load-balancer deletion succeeds (@timoreimann)
* Remove local load-balancer cache entry when load-balancer is deleted (@timoreimann)

## v0.1.15 (beta) - Jun 27th 2019

* Set cloud tagging, authentication lookup skipping, and cloud provider flags in-code (@timoreimann)
* Drop droplet cache usage in Instances implementation (@timoreimann)
* Add note to README about CCM being already installed on DOKS (@snormore)
* Set a custom user agent for the godo client (@andrewsomething)

## v0.1.14 (beta) - Apr 26th 2019

* Update Kubernetes dependencies to 1.14.1 (@timoreimann)
* Handle case where stale droplet cache can result in incorrect node deletions (@nanzhong)

## v0.1.13 (beta) - Apr 3rd 2019

* Add support for configuring a specific vpc id (@nanzhong)

## v0.1.12 (beta) - Mar 26th 2019

* Cache API results for DigitalOcean resources and manage them in ResourcesController (@nanzhong)

## v0.1.11 (beta) - Mar 19th 2019

* loadbalancers: add support for PROXY protocol (@timoreimann)
* loadbalancers: support numeric health check parameters (@timoreimann)

## v0.1.10 (beta) - Feb 26th 2019

* loadbalancers: don't use pointer to loop variable in load balancers map (@bouk)

## v0.1.9 (beta) - Feb 26th 2019

**IMPORTANT:** This release contains a significant bug. Use v0.1.10 instead.

* Reconcile cluster ID tags on DO load-balancer resources (@timoreimann)
* Makefile: Fix check-headers target and header violations (@timoreimann)
* prepend the DO-specific tag component to the cluster ID (@timoreimann)
* add script to clean up used DigitalOcean resources (@timoreimann)
* tag created load balancers with existing cluster ID (@timoreimann)
* add some documentation and fix load balancer naming (@tariq1890)
* bump Go version to 1.11.5 (@timoreimann)
* fix link in docs (@eddiezane)
* fix typo in Makefile (@rig0rmortis)
* remove duplicate 'contributing' section (@groovemonkey)
* add end-to-end test verifying Kubernetes compatibility (@timoreimann)
* support overriding the load-balancer health check protocol via the `service.beta.kubernetes.io/do-loadbalancer-healthcheck-protocol` annotation (@andrewsykim)

## v0.1.8 (beta) - Oct 24th 2018

* add support for loadbalancer health check paths via service annotation `service.beta.kubernetes.io/do-loadbalancer-healthcheck-path` (@andrewsykim)
* various clean ups (golint, CI, etc) (@timoreimann)

## v0.1.7 (beta) - Aug 1st 2018

* implement InstanceShutdownByProviderID which adds taints to droplets that are shutdown (@andrewsykim)

## v0.1.6 (alpha) - May 11th 2018

* support loadbalancer http -> https redirect (@peterver)

## v0.1.5 (alpha) - May 9th 2018

* loadbalancers: Support nodes where nodeName is the private or public IP (@klausenbusk)
* Add the ability to overide the DO API address (@cagedmantis)
* update godo to v1.2.0 (@andrewsykim)
* update kubernetes dependenicies to v1.10.2 (@andrewsykim)

## v0.1.4 (alpha) - March 18th 2018

* Support loadbalancer sticky sessions (@xmudrii)
* Add RBAC ClusterRole, ClusterRoleBindings and ServiceAccount

Supports Kubernetes Versions: v1.8.X - v1.9.X

## v0.1.3 (alpha) - December 13th 2017

* Support clusters where nodeName is the private or public IP (@klausenbusk)
* Switch Docker base image to Alpine from Ubuntu (@klausenbusk)

Supports Kubernetes Versions: v1.8

## v0.1.2 (alpha) - October 5th 2017

* Implement InstanceExistsByProviderID (@andrewsykim)
* Cloud Controller Manager should run as a critical pod with resource requests (@andrewsykim)
* Handle new provider ID format in node spec - digitalocean://droplet-id (@andrewsykim)
* Implement GetZoneByProviderID and GetZoneByNodeName (@bhcleek)
* Remove import for in-tree cloud provider - results in smaller binary (@andrewsykim)

Supports Kubernetes Versions: v1.8

## v0.1.1 (alpha) - September 27th 2017

* Wait for load balancer to be active to retrieve its IP (@odacremolbap)
* Use pagination when listing all droplets (@yuvalsade)

Supports Kubernetes Versions: v1.7

## v0.1.0 (alpha) - August 10th 2017

* implement nodecontroller - responsible for: address managemnet, monitoring node status and node deletions.
* implement zones - responsible for assigning nodes a zone in DigitalOcean
* implement servicecontroller - responsible for: creating, updating and deleting services of type `LoadBalancer` with DO loadbalancers.

Supports Kubernetes Versions: v1.7
