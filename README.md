# Kubernetes Cloud Controller Manager for DigitalOcean

`digitalocean-cloud-controller-manager` is the Kubernetes cloud controller manager implementation for DigitalOcean. Read more about cloud controller managers [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/). Running `digitalocean-cloud-controller-manager` allows you to leverage many of the cloud provider features offered by DigitalOcean on your kubernetes clusters.

WARNING: this project is a work in progress and may not be production ready.

## Requirements

At the current state of Kubernetes, running cloud controller manager requires a few things:

### Version
Kubernetes version 1.7 or greater is required.

### --cloud-provider=external
All kubernetes components specifying a cloud provider should set the flag `--cloud-provider=external`. This is required since the core `kube-controller-manager` will overlap with control loops running against `cloud-controller-manager` unless it is told that an external controller will handle those loops. At the time of writing this, the only components that need updating are:

* kube-apiserver
* kube-controller-manager
* kubelets

WARNING: setting `--cloud-provider=external` will taint all nodes in a cluster with "node.cloudprovider.kubernetes.io/uninitialized", it is the responsibility of cloud controller managers to untaint those nodes once it has finished initializing them. This means that most pods will be left unscheduable until the cloud controller manager is running.

In the future, `--cloud-provider=external` will be the default. Learn more about the future of cloud providers in Kubernetes [here](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/cloud-provider-refactoring.md).

### Node hostnames must match the droplet name
By default, the kubelet will name nodes based on the node's hostname. On DigitalOcean, node hostnames are set based on the name of the droplet. If you decide to override the hostname on kubelets with `--hostname-override`, this will also override the node name in Kubernetes. It is importntant that the node name on Kubernetes matches the droplet name, otherwise cloud controller manager cannot find the corresponding droplet to nodes.

### All droplets must have unique names
All droplet names in kubernetes must be unique since node names in kubernetes must be unique.

## Deployment

### Token
To run digitalocean-cloud-controller-manager, you need a digitalocean access token. If you are already logged in, you can create one [here](https://cloud.digitalocean.com/settings/api/tokens). Ensure the token you create has both read and write access. Once you have an access token, you want to create a Kubernetes Secret.

If your token is `abc123abc123abc123`, then you want to base64 encode your token like so:
```bash
echo -n "abc123abc123abc123" | base64
```

and then insert the base64 encoded token into the secret file [here](https://github.com/digitalocean/digitalocean-cloud-controller-manager/blob/master/releases/token.yml#L10), replacing the placeholder. The secret should look something like this:
```
apiVersion: v1
kind: Secret
metadata:
  name: digitalocean
  namespace: kube-system
data:
  access-token: "YWJjMTIzYWJjMTIzYWJjMTIz"
```

then simply run this command from the root of this repo:
```bash
kubectl apply -f releases/token.yml
```

You should now see the digitalocean secret in the `kube-system` namespace along with other secrets
```bash
$ kubectl -n kube-system get secrets
NAME                  TYPE                                  DATA      AGE
default-token-jskxx   kubernetes.io/service-account-token   3         18h
digitalocean          Opaque                                1         18h
```

### cloud controll manager
Currently we only support alpha release of the `digitalocean-cloud-controller-manager` due to its active development. Run the alpha release like so
```bash
kubectl apply -f releases/alpha.yml
```

## Contributing
At DigitalOcean we value and love our community! If you have any issues or would like to contribute, feel free to open an issue/PR and cc any of the maintainers below.

### Maintainers
* Fatih Arslan - @fatih
* Andrew Sy Kim - @andrewsykim
