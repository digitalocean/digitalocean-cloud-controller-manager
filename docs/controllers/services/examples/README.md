# Load Balancers

DigitalOcean cloud controller manager runs service controller, which is
responsible for watching services of type `LoadBalancer` and creating DO
loadbalancers to satify its requirements. Here are some examples of how it's
used.

## TCP Load Balancer

Not specifying any annotations to your service will create a simple TCP loadbalancer.
Here's an example:

```bash
$ kubectl apply -f tcp-nginx.yml
service "tcp-lb" created
deployment "nginx-example" created
```

By default, kubernetes names load balancers based on the UUID of the service.

```bash
$ doctl compute load-balancer list
ID                                      IP    Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
44ff4bf9-34b9-4461-8442-80a90001cb0e          ace288b51796411e7af69267d6e0a2ca    new       2017-08-04T22:32:33Z    round_robin          tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:tcp,port:31294,path:,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3      entry_protocol:tcp,entry_port:80,target_protocol:tcp,target_port:31294,certificate_id:,tls_passthrough:false
```

## HTTP Load Balancer

Here's an example on how to create a simple http load balancer backed by nginx pods

```bash
$ kubectl apply -f http-nginx.yml
service "http-lb" created
deployment "nginx-example" configured
```

This creates a DO load balancer, using http as the default protocol and using the default load balancing algorithm: round robin.

```bash
$ doctl compute load-balancer list
ID                                      IP                Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
5ceb1d26-e4cf-403b-8677-0e5232eec711    159.203.48.217    aeafab819796311e7af69267d6e0a2ca    active    2017-08-04T22:26:12Z    round_robin          tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:http,port:31018,path:/,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3    entry_protocol:http,entry_port:80,target_protocol:http,target_port:31018,certificate_id:,tls_passthrough:false
```

## HTTP Load Balancer using least connections algorithm

Similar to the previous example, you can change the load balancer algorithm to use least connections instead of round robin by setting the annotation `service.beta.kubernetes.io/do-loadbalancer-algorithm`

```bash
$ kubectl apply -f http-with-least-connections-nginx.yml
service "http-with-least-connections" created
deployment "nginx-example" configured
```

```bash
doctl compute load-balancer list
ID                                      IP    Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
51eec4c9-daa3-4b2b-a96a-2a3f2e18183b          a4a2888aa796411e7af69267d6e0a2ca    new       2017-08-04T22:28:51Z    least_connections    tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:http,port:31320,path:/,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3    entry_protocol:http,entry_port:80,target_protocol:http,target_port:31320,certificate_id:,tls_passthrough:false
```

## HTTPS Load Balancer using a provided certificate

For the sake of example, assume you have a valid key/cert pair for your HTTPS certificate at `key.pem` and `cert.pem`.

Now we can create a certificate to ues for your loadbalancer:

```bash
doctl compute certificate create --name=lb-example --private-key-path=key.pem --leaf-certificate-path=cert.pem
ID                                      Name          SHA-1 Fingerprint                           Expiration Date         Created At
29333fdc-733d-4523-8d79-31acc1544ee0    lb-example    4a55a37f89003a20881e67f1bcc85654fdacc525    2022-07-18T18:46:00Z    2017-08-04T23:01:14Z
```

The next example sets the annotation `service.beta.kubernetes.io/do-loadbalancer-certificate-id` to the certificate ID we want to use, in this case it's `29333fdc-733d-4523-8d79-31acc1544ee0`.

```bash
$ kubectl apply -f https-with-cert-nginx.yml
service "https-with-cert" created
deployment "nginx-example" configured

```

```bash
$ doctl compute load-balancer list
ID                                      IP    Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
20befcaf-533d-4fc9-bc4b-be31f957ad87          a83435adb796911e7af69267d6e0a2ca    new       2017-08-04T23:06:15Z    round_robin          tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:http,port:30361,path:/,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3    entry_protocol:http,entry_port:80,target_protocol:http,target_port:30361,certificate_id:,tls_passthrough:false entry_protocol:https,entry_port:443,target_protocol:http,target_port:32728,certificate_id:29333fdc-733d-4523-8d79-31acc1544ee0,tls_passthrough:false
```

## HTTPS Load Balancer with TLS pass through

You can create a https loadbalancer that will pass encrypted data to your backends instead of doing TLS termination.

```bash
$ kubectl apply -f https-with-pass-through-nginx.yml
service "https-with-tls-pass-through" created
deployment "nginx-example" configured
```

```bash
$ doctl compute load-balancer list
ID                                      IP                Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
105c2071-dcfd-479c-8e84-883d66ba4dec    159.203.52.207    a16bdac1c797611e7af69267d6e0a2ca    active    2017-08-05T00:36:16Z    round_robin          tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:http,port:31644,path:/,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3    entry_protocol:http,entry_port:80,target_protocol:http,target_port:31644,certificate_id:,tls_passthrough:false entry_protocol:https,entry_port:443,target_protocol:https,target_port:30566,certificate_id:,tls_passthrough:true
```

## Accessing pods over a managed load-balancer from inside the cluster

Because of an existing [limitation in upstream Kubernetes](https://github.com/kubernetes/kubernetes/issues/66607), pods cannot talk to other pods via the IP address of an external load-balancer set up through a `LoadBalancer`-typed service. Kubernetes will cause the LB to be bypassed, potentially breaking workflows that expect TLS termination or proxy protocol handling to be applied consistently.

However, a workaround exists that takes advantage of the fact that bypassing only happens when the Service status field returns an IP address but not if it returns a hostname. To leverage it, a DNS record for a custom hostname (at a provider of your choice) must be set up that points to the external IP address of the load-balancer. Afterwards, _digitalocean-cloud-controller-manager_ must be instructed to return the custom hostname (instead of the external LB IP address) in the service ingress status field `status.Hostname` by specifying the hostname in the `service.beta.kubernetes.io/do-loadbalancer-hostname` annotation. Clients may then connect to the hostname to reach the load-balancer from inside the cluster.

To make the load-balancer accessible through multiple hostnames, register additional CNAMEs that all point to the hostname. SSL certificates could then be associated with one or more of these hostnames.

### Setup

The workflow for setting up the `service.beta.kubernetes.io/do-loadbalancer-hostname` annotation is generally:

 1. Deploy the manifest with your Service (example below).
 2. Wait for the service external IP to be available.
 3. Add a DNS record for your hostname pointing to the external IP.
 4. Add the hostname annotation to your manifest (example below). Apply it.

```yaml
kind: Service
apiVersion: v1
metadata:
  name: hello
  annotations:
    service.beta.kubernetes.io/do-loadbalancer-certificate-id: "1234-5678-9012-3456"
    service.beta.kubernetes.io/do-loadbalancer-protocol: "https"
    # Uncomment once hello.example.com points to the external IP address of the DO load-balancer.
    # service.beta.kubernetes.io/do-loadbalancer-hostname: "hello.example.com"
spec:
  type: LoadBalancer
  selector:
    app: hello
  ports:
    - name: https
      protocol: TCP
      port: 443
      targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hello
  template:
    metadata:
      labels:
        app: hello
    spec:
      containers:
      - name: hello
        image: snormore/hello
        ports:
        - containerPort: 80
          protocol: TCP
```

## Changing ownership of a load-balancer (for migration purposes)

In general, multiple services (within the same cluster or across multiple clusters) should not be referencing the same [load-balancer ID](/docs/getting-started.md#load-balancer-id-annotations) to avoid conflicting reconciliation. However, at times this may be desired in order to pass ownership of a load-balancer from one Service to another. A common use case is to migrate a load-balancer from one cluster to a new one in order to preserve its IP address.

This can be safely achieved by first "disowning" the load-balancer from the original Service, which turns all mutating actions (load balancer creates, updates, and deletes) into no-ops. Afterwards, the load-balancer can be taken over by a new Service by setting the `kubernetes.digitalocean.com/load-balancer-id` annotation accordingly.

**Note:** The new Service's cluster must reside in the same VPC as the original Service. Otherwise, ownership cannot be passed on. See the [DigitalOcean VPC documentation](https://www.digitalocean.com/docs/networking/vpc/) for details.

The workflow below outlines the necessary steps.

### Workflow

Suppose you have a Service

```yaml
# For the sake of this example, let's assume the Service lives in cluster "production-v1"
kind: Service
apiVersion: v1
metadata:
  name: app
  annotations:
    kubernetes.digitalocean.com/load-balancer-id: c16b0b29-217b-48eb-907e-93cf2e01fb56
spec:
  selector:
    name: app
  ports:
    - name: http
      protocol: TCP
      port: 80
  type: LoadBalancer
```

in cluster _production-v1_. (Note the `kubernetes.digitalocean.com/load-balancer-id` annotation set by `digitalocean-cloud-controller-managers`.)

Here is how you would go about moving the load-balancer to a new cluster `production-v2`:

1. Make sure that no load-balancer-related errors are reported from `digitalocean-cloud-controller-managers` by checking for the absence of error events in your Service via `kubectl describe service app`. Fix any errors that may be reported to bring the Service into a stable state.
2. Mark the load-balancer as disowned by adding the [`service.kubernetes.io/do-loadbalancer-disown`](docs/controllers/services/annotations.md#servicekubernetesiodo-loadbalancer-disown) annotation and assigning it a `"true"` value. Afterwards, the Service object should look like this:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: app
  annotations:
    kubernetes.digitalocean.com/load-balancer-id: c16b0b29-217b-48eb-907e-93cf2e01fb56
    service.kubernetes.io/do-loadbalancer-disown: "true"
spec:
  selector:
    name: app
  ports:
    - name: http
      protocol: TCP
      port: 80
  type: LoadBalancer
```

3. Make sure the change is applied correctly by checking the Service events again. Once applied, all mutating requests directed at the load-balancer and driven through the Service object will be ignored.

(At this point, you could already delete the Service while the corresponding load-balancer would not be deleted.)

4. In our hypthetical new cluster _production-v2_, create a new Service that is supposed to take over ownership and **set the `kubernetes.digitalocean.com/load-balancer-id` annotation right from the start**:

```yaml
# This would be in cluster "production-v2"
kind: Service
apiVersion: v1
metadata:
  name: app
  annotations:
    kubernetes.digitalocean.com/load-balancer-id: c16b0b29-217b-48eb-907e-93cf2e01fb56
spec:
  selector:
    name: app
  ports:
    - name: http
      protocol: TCP
      port: 80
  type: LoadBalancer
```

(In the real-world, your Service may have a number of load-balancer-specific configuration annotation which you would transfer to the new Service as well.)

5. Wait for `digitalocean-cloud-controller-managers` to finish reconciliation by checking the Service events until things have settled down.

At this point, the load-balancer should be owned by the new Service, meaning that traffic should be routed into the new cluster.

6. If you have not done so already: delete the Service _in the old cluster_ (or change its [type to something other than `LoadBalancer`](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) if you want to, say, continue using the Service for cluster-internal routing).

You're done!

## Excluding specific nodes as targets

To exclude a particular node from the load balancer target set, add the `node.kubernetes.io/exclude-from-external-load-balancers=` label to the node. (The value is not relevant.) This requires the `ServiceNodeExclusion` [feature gate](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/) to be enabled, which is the default since Kubernetes 1.19.

**Note:** For services that use `externalTrafficPolicy=Local`, this may mean that any pods on excluded nodes are not reachable by those external load-balancers.

## Surfacing errors related to provisioning a load balancer

Cloud Controller Manager is using DigitalOcean API internally to provision a
DigitalOcean load balancer. It might happen that provisioning will be
unsuccessful, because of various reasons.

You can find provisioning status and all the reconciliation errors that Cloud
Controller Manager encountered during the LoadBalancer service life-cycle in
the service's event stream.

Provided that your kubernetes service name is `my-svc`, you can use following
commands to access events attached to this object:

```bash
$ kubectl describe service my-svc
```

In the above case, events should be available at the bottom of the description,
if present.

To get only events for a given service, you can use following command:

```bash
$ kubectl get events --field-selector involvedObject.name=my-svc
```
