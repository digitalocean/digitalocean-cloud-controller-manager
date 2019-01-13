# Service Annotations

DigitalOcean cloud controller manager watches for Services of type `LoadBalancer` and will create corresponding DigitalOcean Load Balancers matching the Kubernetes service. The Load Balancer can be configured by applying annotations to the Service resource. The following annotations can be used:

### service.beta.kubernetes.io/do-loadbalancer-protocol

The default protocol for DigitalOcean Load Balancers. Ports specified in the annotation `service.beta.kubernetes.io/do-loadbalancer-tls-ports` will be overwritten to https. Options are `tcp`, `http` and `https`. Defaults to `tcp`.

### service.beta.kubernetes.io/do-loadbalancer-healthcheck-path

The path used to check if a backend droplet is healthy. Defaults to "/".

### service.beta.kubernetes.io/do-loadbalancer-healthcheck-protocol

The health check protocol to use to check if a backend droplet is healthy. Defaults to the protocol used in `service.beta.kubernetes.io/do-loadbalancer-protocol`. Options are `tcp` and `http`.

### service.beta.kubernetes.io/do-loadbalancer-tls-ports

Specify which ports of the loadbalancer should use the https protocol. This is a comma separated list of ports (e.g. 443,6443,7443).

### service.beta.kubernetes.io/do-loadbalancer-tls-passthrough

Specify whether the DigitalOcean Load Balancer should pass encrypted data to backend droplets. This is optional. Options are `true` or `false`. Defaults to `false`.

### service.beta.kubernetes.io/do-loadbalancer-certificate-id

Specifies the certificate ID used for https. This annotation is required if `service.beta.kubernetes.io/do-loadbalancer-tls-ports` is used. To list available certificates and their IDs, use `doctl compute certificate list` or find it in the [control panel](https://cloud.digitalocean.com/account/security).

### service.beta.kubernetes.io/do-loadbalancer-algorithm

Specifies which algorithm the Load Balancer should use. Options are `round_robin`, `least_connections`. Defaults to `round_robin`.

### service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type

Specifies which stick session type the loadbalancer should use. Options are `none` or `cookies`.

### service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-cookie-ttl

Specifies the TTL of cookies used for loadbalancer sticky sessions. This annotation is required if `service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type` is set.

### service.beta.kubernetes.io/do-loadbalancer-redirect-http-to-https

Indiciates whether or not http traffic should be redirected to https. Options are `true` or `false`. Defaults to `false`.


See example Kubernetes Services using LoadBalancers [here](examples/).
