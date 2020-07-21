# Getting Started

## Requirements

At the current state of Kubernetes, running cloud controller manager requires a few things. Please read through the requirements carefully as they are critical to running cloud controller manager on a Kubernetes cluster on DigtialOcean.

### Version

These are the recommended versions to run the cloud controller manager based on your Kubernetes version

* Use CCM versions <= v0.1.1 if you're running Kubernetes version v1.7
* Use CCM versions >= v0.1.2 if you're running Kubernetes version v1.8
* Use CCM versions >= v0.1.4 if you're running Kubernetes version v1.9 - v1.10
* Use CCM versions >= v0.1.5 if you're running Kubernetes version >= v1.10
* Use CCM versions >= v0.1.8 if you're running Kubernetes version >= v1.11

### Parameters

This section outlines parameters that can be passed to the cloud controller manager binary.

#### --cloud-provider=external

All `kubelet`s in your cluster **MUST** set the flag `--cloud-provider=external`. `kube-apiserver` and `kube-controller-manager` must **NOT** set the flag `--cloud-provider` which will default them to use no cloud provider natively.

**WARNING**: setting `--cloud-provider=external` will taint all nodes in a cluster with `node.cloudprovider.kubernetes.io/uninitialized`, it is the responsibility of cloud controller managers to untaint those nodes once it has finished initializing them. This means that most pods will be left unscheduable until the cloud controller manager is running.

In the future, `--cloud-provider=external` will be the default. Learn more about the future of cloud providers in Kubernetes [here](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/cloud-provider/cloud-provider-refactoring.md).

#### --provider-id=digitalocean://\<droplet ID\>

A so-called _provider ID_ annotation is attached to each node by the cloud controller manager that serves as a unique identifier to the cloud-specific VM representation. With DigitalOcean, the droplet ID is used for this purpose. The provider ID can be leveraged for efficient droplet lookups via the DigitalOcean API. Lacking the provider ID, a name-based lookup is mandated by the cloud provider interface. However, this is fairly expensive at DigitalOcean since the API does not support droplet retrieval based on names, meaning that the DigitalOcean cloud controller manager needs to iterate over all available droplets to find the one matching the desired name.

For DigitalOcean accounts with small to medium numbers of droplets, iteration should not be much of an issue. However, accounts having a large number of droplets (at the order of hundreds) would likely experience observable performance degradations.

A solution to the problem is to pass the provider ID (aka droplet ID) via the kubelet on bootstrap through the `--provider-id=digitalocean://<droplet ID>` parameter. The droplet ID can be retrieved from the metadata service available on each droplet at the `http://169.254.169.254/metadata/v1/id` address. Passing the provider ID like this causes the node annotation to be available right from the start, thereby enabling fast API lookups for droplets at all times.

DigitalOcean's managed Kubernetes offering DOKS sets the provider ID on each worker node kubelet instance.

### Managed firewall handling for public access

`digitalocean-cloud-controller-manager` can manage a dedicated [DigitalOcean Cloud Firewall](https://www.digitalocean.com/docs/networking/firewalls/) to dynamically allow access to NodePorts. A controller watches over Services and modifies the inbound rules of a firewall to permit access to the target NodePorts, and likewise close down access again if a Service is deleted or its type changed to something other than `NodePort`. (Note that DigitalOcean Load-Balancers access the cluster over the VPC interface and as such are not managed by this particular firewall for now.)

By default, no firewall will be managed. To enable firewall management, the following environment variables need to be defined:

* `PUBLIC_ACCESS_FIREWALL_NAME`: the name of the firewall to use.
* `PUBLIC_ACCESS_FIREWALL_TAGS`: a comma-separated list of tags that match the worker droplets the firewall should target.

Managed firewalls should **not** be modified directly as such changes will be reverted eventually, including re-creation of the firewall should it ever be found missing.

If management of the firewall is not desired anymore, the environment variables must be unset before the firewall can be deleted by the user manually.

### DEBUG_ADDR environment variable

If the `DEBUG_ADDR` environment variable is specified, then an HTTP server is started on the given address (e.g., `:12301`). It serves on `/healthz` and queries the `/v2/account` path of the DigitalOcean API on request.

The purpose of this endpoint is to check the availability of the DigitalOcean API on demand from the perspective of the cloud controller manager.

### Kubernetes node name overriding

By default, the kubelet will name nodes based on the node's hostname. On DigitalOcean, node hostnames are set based on the name of the droplet. If you decide to override the hostname on kubelets with `--hostname-override`, this will also override the node name in Kubernetes.

Overriding the hostname is okay if provider IDs are injected by the kubelet. (See the previous section.) If that is not the case or there are nodes lacking the provider ID, however, then the Kubenretes node name must match either the droplet name, private ipv4 IP, or the public ipv4 IP. Otherwise, `cloud-controller-manager` won't be able to find the corresponding droplets in the DigitalOcean API and consequently fail to bootstrap nodes.

### Kubernetes nodes can be reached via IP address only

When setting the droplet host name as the node name (which is the default), Kubernetes will try to reach the node using its host name. However, this won't work since host names aren't resovable on DO. For example, when you run `kubectl logs` you will get an error like so:

```bash
$ kubectl logs -f mypod
Error from server: Get https://k8s-worker-03:10250/containerLogs/default/mypod/mypod?follow=true: dial tcp: lookup k8s-worker-03 on 67.207.67.3:53: no such host
```

Since on DigitalOcean the droplet's name is not resolvable, it's important to tell the Kubernetes masters to use another address type to reach its workers. You can do this by setting `--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname` on the apiserver. Doing this will tell Kubernetes to use a droplet's private IP to connect to the node before attempting it's public IP and then it's host name.

### All droplets must have unique names

All droplet names in kubernetes must be unique since node names in kubernetes must be unique.

## Implementation Details

Currently `digitalocean-cloud-controller-manager` implements:

* nodecontroller - updates nodes with cloud provider specific labels and addresses, also deletes kubernetes nodes when deleted on the cloud provider.
* servicecontroller - responsible for creating LoadBalancers when a service of `Type: LoadBalancer` is created in Kubernetes.

In the future, it may implement:

* routecontroller - responsible for creating firewall rules

### Resource Tagging

When the environment variable `DO_CLUSTER_ID` is given, `digitalocean-cloud-controller-manager` will use it to tag DigitalOcean resources additionally created during runtime (such us load-balancers) accordingly. The cloud ID is usually represented by a UUID and prefixed with `k8s:` when tagging, e.g., `k8s:c63024c5-adf7-4459-8547-9c0501ad5a51`.

The primary purpose of the variable is to allow DigitalOcean customers to easily understand which resources belong to the same DOKS cluster. Specifically, it is not needed (nor helpful) to have in DIY cluster installations.

### Custom VPC

When a cluster is created in a non-default VPC for the region, the environment variable `DO_CLUSTER_VPC_ID` must be specified or Load Balancer creation for services will fail.

### Load-balancer ID annotations

`digitalocean-cloud-controller-manager` attaches the UUID of load-balancers to the corresponding Service objects (given they are of type `LoadBalancer`) using the `kubernetes.digitalocean.com/load-balancer-id` annotation. This serves two purposes:

1. To support load-balancer renames.
2. To efficiently look up load-balancer resources in the DigitalOcean API.

`digitalocean-cloud-controller-manager` annotates new and existing Services. Note that a load-balancer that is renamed before the annotation is added will be lost, and a new one will be created.

You can have `digitalocean-cloud-controller-manager` manage an existing load-balancer by creating a `LoadBalancer` Service annotated with the UUID of the load-balancer. However, if it is already managed by another Service/cluster, you have to make sure [to disown it properly](/docs/controllers/services/examples/README.md#changing-ownership-of-a-load-balancer-for-migration-purposes) to prevent conflicting modifications to the load-balancer.

## Deployment

### Token

To run `digitalocean-cloud-controller-manager`, you need a DigitalOcean personal access token. If you are already logged in, you can create one [here](https://cloud.digitalocean.com/settings/api/tokens). Ensure the token you create has both read and write access. Once you have a personal access token, create a Kubernetes Secret as a way for the cloud controller manager to access your token. You can do this with one of the following methods:

#### Script

You can use the script [scripts/generate-secret.sh](https://github.com/digitalocean/digitalocean-cloud-controller-manager/blob/master/scripts/generate-secret.sh) in this repo to create the Kubernetes Secret. Note that this will apply changes using your default `kubectl` context. For example, if your token is `abc123abc123abc123`, run the following to create the Kubernetes Secret.

```bash
export DIGITALOCEAN_ACCESS_TOKEN=abc123abc123abc123
scripts/generate-secret.sh
```

#### Manually

Copy [releases/secret.yml.tmpl](https://github.com/digitalocean/digitalocean-cloud-controller-manager/blob/master/releases/secret.yml.tmpl) to releases/secret.yml:

```bash
cp releases/secret.yml.tmpl releases/secret.yml
```

Replace the placeholder in the copy with your token. When you're done, the releases/secret.yml should look something like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: digitalocean
  namespace: kube-system
stringData:
  access-token: "abc123abc123abc123"
```

Finally, run this command from the root of this repo:

```bash
kubectl apply -f releases/secret.yml
```

You should now see the digitalocean secret in the `kube-system` namespace along with other secrets

```bash
$ kubectl -n kube-system get secrets
NAME                  TYPE                                  DATA      AGE
default-token-jskxx   kubernetes.io/service-account-token   3         18h
digitalocean          Opaque                                1         18h
```

### Cloud controller manager

Currently we only support alpha release of the `digitalocean-cloud-controller-manager` due to its active development. Run the first alpha release like so

```bash
kubectl apply -f releases/v0.1.26.yml
deployment "digitalocean-cloud-controller-manager" created
```

NOTE: the deployments in `releases/` are meant to serve as an example. They will work in a majority of cases but may not work out of the box for your cluster.
