# Service Annotations

DigitalOcean cloud controller manager watches for Services of type `LoadBalancer` and will create corresponding DigitalOcean Load Balancers matching the Kubernetes service. The Load Balancer can be configured by applying annotations to the Service resource. The annotations listed below can be used.

See example Kubernetes Services using LoadBalancers [here](examples/).

## service.beta.kubernetes.io/do-loadbalancer-name

Specifies a custom name for the Load Balancer. Existing Load Balancers will be renamed. The name must adhere to the following rules:

- it must not be longer than 255 characters
- it must start with an alphanumeric character
- it must consist of alphanumeric characters or the '.' (dot) or '-' (dash) characters
- except for the final character which must not be '-' (dash)

If no custom name is specified, a default name is chosen consisting of the character `a` appended by the Service UID.

## service.beta.kubernetes.io/do-loadbalancer-protocol

The default protocol for DigitalOcean Load Balancers. Options are `tcp`, `http`, `https`, `http2`, `http3`. Defaults to `tcp`.

Certain annotations may override the default protocol. See the more specific descriptions below.

If `https`, `http2`, or `http3` is specified, then either `service.beta.kubernetes.io/do-loadbalancer-certificate-id` or `service.beta.kubernetes.io/do-loadbalancer-tls-passthrough` must be specified as well.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-port

The service port used to check if a backend droplet is healthy. Defaults to the first port in a service.

**Note:** Users must specify a port exposed by the Service, not the NodePort.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-path

The path used to check if a backend droplet is healthy. Defaults to "/".

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-protocol

The health check protocol to use to check if a backend droplet is healthy. Defaults to `tcp` if not specified. Options are `tcp`, `http`, and `https`.

The first node port on the service is used as health check port.

Note that while UDP is not a supported healthcheck protocol, if your load balancer has UDP service ports you must configure a TCP service as a health check for the load balancer to work properly.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-check-interval-seconds

The number of seconds between between two consecutive health checks. The value must be between 3 and 300. If not specified, the default value is 3.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-response-timeout-seconds

The number of seconds the Load Balancer instance will wait for a response until marking a health check as failed. The value must be between 3 and 300. If not specified, the default value is 5.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-unhealthy-threshold

The number of times a health check must fail for a backend Droplet to be marked "unhealthy" and be removed from the pool for the given service. The vaule must be between 2 and 10. If not specified, the default value is 3.

## service.beta.kubernetes.io/do-loadbalancer-healthcheck-healthy-threshold

The number of times a health check must pass for a backend Droplet to be marked "healthy" for the given service and be re-added to the pool. The vaule must be between 2 and 10. If not specified, the default value is 5.

## service.beta.kubernetes.io/do-loadbalancer-http-ports

Specify which ports of the loadbalancer should use the HTTP protocol. This is a comma separated list of ports (e.g. 80,8080).

Ports must not be shared between this annotation, `service.beta.kubernetes.io/do-loadbalancer-tls-ports`, `service.beta.kubernetes.io/do-loadbalancer-http2-ports`, and `service.beta.kubernetes.io/do-loadbalancer-http3-port`.

## service.beta.kubernetes.io/do-loadbalancer-tls-ports

Specify which ports of the loadbalancer should use the HTTPS protocol. This is a comma separated list of ports (e.g. 443,6443,7443).

If specified, exactly one of `service.beta.kubernetes.io/do-loadbalancer-tls-passthrough` and `service.beta.kubernetes.io/do-loadbalancer-certificate-id` must also be provided.

If no HTTPS port is specified but one of `service.beta.kubernetes.io/do-loadbalancer-tls-passthrough` or `service.beta.kubernetes.io/do-loadbalancer-certificate-id` is, then port 443 is assumed to be used for HTTPS. This does not hold if `service.beta.kubernetes.io/do-loadbalancer-http2-ports` or `service.beta.kubernetes.io/do-loadbalancer-http3-port` already specifies 443.

Ports must not be shared between this annotation, `service.beta.kubernetes.io/do-loadbalancer-http-ports`, `service.beta.kubernetes.io/do-loadbalancer-http2-ports`, and `service.beta.kubernetes.io/do-loadbalancer-http3-port`.

## service.beta.kubernetes.io/do-loadbalancer-http2-ports

Specify which ports of the loadbalancer should use the HTTP2 protocol. This is a comma separated list of ports (e.g. 443,6443,7443).

If specified, exactly one of `service.beta.kubernetes.io/do-loadbalancer-tls-passthrough` and `service.beta.kubernetes.io/do-loadbalancer-certificate-id` must also be provided.

The annotation is required for implicit HTTP2 usage, i.e., when `service.beta.kubernetes.io/do-loadbalancer-protocol` is not set to `http2`. (Unlike `service.beta.kubernetes.io/do-loadbalancer-tls-ports`, no default port is assumed for HTTP2 in order to retain compatibility with the semantics of implicit HTTPS usage.)

Ports must not be shared between this annotation, `service.beta.kubernetes.io/do-loadbalancer-http-ports`, `service.beta.kubernetes.io/do-loadbalancer-http3-port`, and `service.beta.kubernetes.io/do-loadbalancer-tls-ports`.

## service.beta.kubernetes.io/do-loadbalancer-http3-port

Specify which port of the loadbalancer should use the HTTP3 protocol. Unlike other annotations, this is a single value, NOT multiple comma-separated values.

If specified, `service.beta.kubernetes.io/do-loadbalancer-certificate-id` must also be provided. In addition, either `service.beta.kubernetes.io/do-loadbalancer-tls-ports` OR `service.beta.kubernetes.io/do-loadbalancer-http2-ports` must be provided.

The annotation is required for implicit HTTP3 usage, i.e., when `service.beta.kubernetes.io/do-loadbalancer-protocol` is not set to `http3`. (Unlike `service.beta.kubernetes.io/do-loadbalancer-tls-ports`, no default port is assumed for HTTP3 in order to retain compatibility with the semantics of implicit HTTPS usage.)

## service.beta.kubernetes.io/do-loadbalancer-tls-passthrough

Specify whether the DigitalOcean Load Balancer should pass encrypted data to backend droplets. This is optional. Options are `"true"` or `"false"`. Defaults to `"false"`.

**Note**

You have to supply the value as string (ex. `"true"`, not `true`), otherwise you might run into a [k8s bug that throws away all annotations on your `Service` resource](https://github.com/kubernetes/kubernetes/issues/59113).

## service.beta.kubernetes.io/do-loadbalancer-certificate-id

Specifies the certificate ID used for https. To list available certificates and their IDs, install [doctl](https://github.com/digitalocean/doctl) and run `doctl compute certificate list`.

## service.beta.kubernetes.io/do-loadbalancer-hostname

Specifies the hostname used for the Service `status.Hostname` instead of assigning `status.IP` directly. This can be used to workaround the issue of [kube-proxy adding external LB address to node local iptables rule](https://github.com/kubernetes/kubernetes/issues/66607), which will break requests to an LB from in-cluster if the LB is expected to terminate SSL or proxy protocol. See the [examples/README](examples/README.md) for more detail.

## service.beta.kubernetes.io/do-loadbalancer-algorithm

Specifies which algorithm the Load Balancer should use. Options are `round_robin`, `least_connections`. Defaults to `round_robin`.

## service.beta.kubernetes.io/do-loadbalancer-size-slug

Specifies which size the Load Balancer should be created with. Options are `lb-small`, `lb-medium`, and `lb-large`. Only one of `do-loadbalancer-size-slug` and `do-loadbalancer-size-unit` can be specified. Defaults to `lb-small`.

DEPRECATED: Use `do-loadbalancer-size-unit` instead.

## "service.beta.kubernetes.io/do-loadbalancer-size-unit"

Specifies the number of nodes to create the load balancer with. Options are numbers greater than or equal to `1`. Only one of `do-loadbalancer-size-unit` and `do-loadbalancer-size-slug` can be specified. Defaults to `1`.

## service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type

Specifies which stick session type the loadbalancer should use. Options are `none` or `cookies`.

**Note**
 - Sticky sessions will route consistently to the same nodes, not pods, so you should avoid having more than one pod per node serving requests.
 - Sticky sessions require your Service to configure `externalTrafficPolicy: Local` to avoid NAT confusion on the way in.
 - Using sticky sessions with only a TCP forwarding rule will not work as expected. Sticky sessions requires HTTP to function properly.

## service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-cookie-name

Specifies what cookie name to use for the DO load balancer sticky session. This annotation is required if `service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type` is set to `cookies`.

## service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-cookie-ttl

Specifies the TTL of cookies used for loadbalancer sticky sessions. This annotation is required if `service.beta.kubernetes.io/do-loadbalancer-sticky-sessions-type` is set to `cookies`.

## service.beta.kubernetes.io/do-loadbalancer-redirect-http-to-https

Indicates whether or not http traffic should be redirected to https. Options are `"true"` or `"false"`. Defaults to `"false"`. 

**Note**

You have to supply the value as string (ex. `"true"`, not `true`), otherwise you might run into a [k8s bug that throws away all annotations on your `Service` resource](https://github.com/kubernetes/kubernetes/issues/59113).

## service.beta.kubernetes.io/do-loadbalancer-disable-lets-encrypt-dns-records

Specifies whether automatic DNS record creation should be disabled when a Let's Encrypt cert is added to a load balancer. Defaults to `"false"`.

## service.beta.kubernetes.io/do-loadbalancer-enable-proxy-protocol

Indicates whether PROXY protocol should be enabled. Options are `"true"` or `"false"`. Defaults to `"false"`.

**Note**

You have to supply the value as string (ex. `"true"`, not `true`), otherwise you might run into a [k8s bug that throws away all annotations on your `Service` resource](https://github.com/kubernetes/kubernetes/issues/59113).

## service.beta.kubernetes.io/do-loadbalancer-enable-backend-keepalive

Indicates whether HTTP keepalive connections should be enabled to backend target droplets. Options are `"true"` or `"false"`. Defaults to `"false"`.

**Note**

You have to supply the value as string (ex. `"true"`, not `true`), otherwise you might run into a [k8s bug that throws away all annotations on your `Service` resource](https://github.com/kubernetes/kubernetes/issues/59113).

## service.kubernetes.io/do-loadbalancer-disown

Indicates whether the managed load-balancer should be disowned. Disowned load-balancers are not mutated anymore, including creates, updates, and deletes. This can be employed to manage the load-balancer by a different cluster. Options are `"true"` or `"false"`. Defaults to `"false"`.

**Warning** Disowned load-balancers do not necessarily work correctly anymore because needed load-balancer updates (in terms of target nodes or configuration annotations) stop being propagated to the DigitalOcean load-balancer. Similarly, the Service status field may not reflect the right state anymore. Consequently, users should assign disowned load-balancers to a new Service without much delay.

See also the [elaborate example](/docs/controllers/services/examples/README.md#changing-ownership-of-a-load-balancer-for-migration-purposes).

**Note**

You have to supply the value as string (ex. `"true"`, not `true`), otherwise you might run into a [k8s bug that throws away all annotations on your `Service` resource](https://github.com/kubernetes/kubernetes/issues/59113).
