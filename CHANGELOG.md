# CHANGELOG

* Support HTTPS as health check protocol (@timoreimann)
* add controller to manage worker firewall for public access (@MorrisLaw)

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
