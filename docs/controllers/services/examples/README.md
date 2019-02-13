# Load Balancers

DigitalOcean cloud controller manager runs service controller, which is responsible for watching services of type `LoadBalancer` and creating DO loadbalancers to satify its requirements. Here are some examples of how it's used.

## TCP Load Balancer
Not specifying any annotations to your service will create a simple TCP loadbalancer.
Here's an example:
```
$ kubectl apply -f tcp-nginx.yml
service "tcp-lb" created
deployment "nginx-example" created
```

By default, kubernetes names load balancers based on the UUID of the service.
```
$ doctl compute load-balancer list
ID                                      IP    Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
44ff4bf9-34b9-4461-8442-80a90001cb0e          ace288b51796411e7af69267d6e0a2ca    new       2017-08-04T22:32:33Z    round_robin          tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:tcp,port:31294,path:,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3      entry_protocol:tcp,entry_port:80,target_protocol:tcp,target_port:31294,certificate_id:,tls_passthrough:false
```


## HTTP Load Balancer.
Here's an example on how to create a simple http load balancer backed by nginx pods
```bash
$ kubectl apply -f http-nginx.yml
service "http-lb" created
deployment "nginx-example" configured
```

This creates a DO load balancer, using http as the default protocol and using the default load balancing algorithm: round robin.

```
$ doctl compute load-balancer list
ID                                      IP                Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
5ceb1d26-e4cf-403b-8677-0e5232eec711    159.203.48.217    aeafab819796311e7af69267d6e0a2ca    active    2017-08-04T22:26:12Z    round_robin          tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:http,port:31018,path:/,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3    entry_protocol:http,entry_port:80,target_protocol:http,target_port:31018,certificate_id:,tls_passthrough:false
```

## HTTP Load Balancer using least connections algorithm
Similar to the previous example, you can change the load balancer algorithm to use least connections instead of round robin by setting the annotation `service.beta.kubernetes.io/do-loadbalancer-algorithm`
```
$ kubectl apply -f http-with-least-connections-nginx.yml
service "http-with-least-connections" created
deployment "nginx-example" configured
```

```
doctl compute load-balancer list
ID                                      IP    Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
51eec4c9-daa3-4b2b-a96a-2a3f2e18183b          a4a2888aa796411e7af69267d6e0a2ca    new       2017-08-04T22:28:51Z    least_connections    tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:http,port:31320,path:/,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3    entry_protocol:http,entry_port:80,target_protocol:http,target_port:31320,certificate_id:,tls_passthrough:false
```

## HTTPS Load Balancer using a provided certificate
For the sake of example, assume you have a valid key/cert pair for your HTTPS certificate at `key.pem` and `cert.pem`.

Now we can create a certificate to ues for your loadbalancer:
```
doctl compute certificate create --name=lb-example --private-key-path=key.pem --leaf-certificate-path=cert.pem
ID                                      Name          SHA-1 Fingerprint                           Expiration Date         Created At
29333fdc-733d-4523-8d79-31acc1544ee0    lb-example    4a55a37f89003a20881e67f1bcc85654fdacc525    2022-07-18T18:46:00Z    2017-08-04T23:01:14Z
```

The next example sets the annotation `service.beta.kubernetes.io/do-loadbalancer-certificate-id` to the certificate ID we want to use, in this case it's `29333fdc-733d-4523-8d79-31acc1544ee0`.

```
$ kubectl apply -f https-with-cert-nginx.yml
service "https-with-cert" created
deployment "nginx-example" configured

```

```
$ doctl compute load-balancer list
ID                                      IP    Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
20befcaf-533d-4fc9-bc4b-be31f957ad87          a83435adb796911e7af69267d6e0a2ca    new       2017-08-04T23:06:15Z    round_robin          tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:http,port:30361,path:/,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3    entry_protocol:http,entry_port:80,target_protocol:http,target_port:30361,certificate_id:,tls_passthrough:false entry_protocol:https,entry_port:443,target_protocol:http,target_port:32728,certificate_id:29333fdc-733d-4523-8d79-31acc1544ee0,tls_passthrough:false
```

## HTTPS Load Balancer with TLS pass through
You can create a https loadbalancer that will pass encrypted data to your backends instead of doing TLS termination.
```
$ kubectl apply -f https-with-pass-through-nginx.yml
service "https-with-tls-pass-through" created
deployment "nginx-example" configured
```

```
$ doctl compute load-balancer list
ID                                      IP                Name                                Status    Created At              Algorithm            Region    Tag    Droplet IDs                   SSL      Sticky Sessions                                Health Check                                                                                                                     Forwarding Rules
105c2071-dcfd-479c-8e84-883d66ba4dec    159.203.52.207    a16bdac1c797611e7af69267d6e0a2ca    active    2017-08-05T00:36:16Z    round_robin          tor1             55581290,55581291,55581292    false    type:none,cookie_name:,cookie_ttl_seconds:0    protocol:http,port:31644,path:/,check_interval_seconds:3,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3    entry_protocol:http,entry_port:80,target_protocol:http,target_port:31644,certificate_id:,tls_passthrough:false entry_protocol:https,entry_port:443,target_protocol:https,target_port:30566,certificate_id:,tls_passthrough:true

```
