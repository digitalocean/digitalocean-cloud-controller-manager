# Service Annotations

DigitalOcean cloud controller manager watches for Services of type `LoadBalancer` and will create corresponding DigitalOcean Load Balancers matching the Kubernetes service. The Load Balancer can be configured by applying annotations to the Service resource. The annotations listed below can be used.

See example Kubernetes Services using LoadBalancers [here](examples/).

## service.beta.kubernetes.io/do-loadbalancer-protocol

The default protocol for DigitalOcean Load Balancers. Ports specified in the annotation `service.beta.kubernetes.io/do-loadbalancer-tls-ports` will be overwritten to https. Options are `tcp`, `http` and `https`. Defaults to `tcp`.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-path

The path used to check if a backend droplet is healthy. Defaults to "/".

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-protocol

The health check protocol to use to check if a backend droplet is healthy. Defaults to the protocol used in `service.beta.kubernetes.io/do-loadbalancer-protocol`. Options are `tcp` and `http`.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-check-interval-seconds

The number of seconds between between two consecutive health checks. The value must be between 3 and 300. If not specified, the default value is 3.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-response-timeout-seconds

The number of seconds the Load Balancer instance will wait for a response until marking a health check as failed. The value must be between 3 and 300. If not specified, the default value is 5.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-unhealthy-threshold

The number of times a health check must fail for a backend Droplet to be marked "unhealthy" and be removed from the pool for the given service. The vaule must be between 2 and 10. If not specified, the default value is 3.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-healthy-threshold

The number of times a health check must pass for a backend Droplet to be marked "healthy" for the given service and be re-added to the pool. The vaule must be between 2 and 10. If not specified, the default value is 5.

## service.beta.kubernetes.io/do-loadbalancer-tls-ports

Specify which ports of the loadbalancer should use the https protocol. This is a comma separated list of ports (e.g. 443,6443,7443).
If specified, exactly one of `service.beta.kubernetes.io/do-loadbalancer-tls-passthrough` and `service.beta.kubernetes.io/do-loadbalancer-certificate-id` must also be provided.

## service.beta.kubernetes.io/do-loadbalancer-tls-passthrough

Specify whether the DigitalOcean Load Balancer should pass encrypted data to backend droplets. This is optional. Options are `true` or `false`. Defaults to `false`.
If enabled, ports specified in `service.beta.kubernetes.io/do-loadbalancer-tls-ports` will use https, or 443 if none are given.

## service.beta.kubernetes.io/do-loadbalancer-certificate-id

Specifies the certificate ID used for https. To list available certificates and their IDs, install [doctl](https://github.com/digitalocean/doctl) and run `doctl compute certificate list`.
If enabled, ports specified in `service.beta.kubernetes.io/do-loadbalancer-tls-ports` will use https, or 443 if none are given.

## service.beta.kubernetes.io/do-loadbalancer-algorithm

Specifies which algorithm the Load Balancer should use. Options are `round_robin`, `least_connections`. Defaults to `round_robin`.

## service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type

Specifies which stick session type the loadbalancer should use. Options are `none` or `cookies`.

## service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-cookie-name

Specifies what cookie name to use for the DO load balancer sticky session. This annotation is required if `service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type` is set to `cookies`.

## service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-cookie-ttl

Specifies the TTL of cookies used for loadbalancer sticky sessions. This annotation is required if `service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type` is set to `cookies`.

## service.beta.kubernetes.io/do-loadbalancer-redirect-http-to-https

Indicates whether or not http traffic should be redirected to https. Options are `true` or `false`. Defaults to `false`.

## service.beta.kubernetes.io/do-loadbalancer-enable-proxy-protocol

Indicates whether PROXY protocol should be enabled. Options are `true` or `false`. Defaults to `false`.
