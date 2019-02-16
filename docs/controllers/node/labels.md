# Node Labels

Nodes registered with the DigitalOcean cloud controller manager should have the following labels:

## beta.kubernetes.io/instance-type

Defines the instance type using the droplet size slug. For example, a standard 2 vCPU droplet with 4 GB of memory would have label `beta.kubernetes.io/instance-type: s-2vcpu-4gb`. You can see all available sizes in the [API docs](https://developers.digitalocean.com/documentation/v2/#list-all-sizes).

## failure-domain.beta.kubernetes.io/region

Defines the region a node is running in. For example, a droplet running in tor1 will have label `failure-domain.beta.kubernetes.io/region: tor1`.
