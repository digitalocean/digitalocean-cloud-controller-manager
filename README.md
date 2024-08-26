# Kubernetes Cloud Controller Manager for DigitalOcean

`digitalocean-cloud-controller-manager` is the Kubernetes cloud controller manager implementation for DigitalOcean. Read more about cloud controller managers [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/). Running `digitalocean-cloud-controller-manager` allows you to leverage many of the cloud provider features offered by DigitalOcean on your Kubernetes clusters.

## Releases

Cloud Controller Manager follows [semantic versioning](https://semver.org/).
Although the version is still below v1, the project is considered
production-ready.

Because of the fast Kubernetes release cycles, CCM (Cloud Controller Manager)
will **only** support the version that is _also_ supported on [DigitalOcean Kubernetes
product](https://www.digitalocean.com/products/kubernetes/). Any other releases
will be not officially supported by us.

## Getting Started

Learn more about running DigitalOcean cloud controller manager [here](docs/getting-started.md)!

_Note that this CCM is installed by default on [DOKS](https://www.digitalocean.com/products/kubernetes/) (DigitalOcean Managed Kubernetes), you don't have to do it yourself._

## Examples

Here are some examples of how you could leverage `digitalocean-cloud-controller-manager`:

* [loadbalancers](docs/controllers/services/examples/)
* [node labels and addresses](docs/controllers/node/examples/)

## Production notes

### do not modify DO load-balancers manually

When you are creating load-balancers through CCM (via `LoadBalancer`-typed Services),it is very important that you **must not change the DO load-balancer configuration manually.** Such changes will eventually be reverted by the reconciliation loop built into CCM. There is one exception in load-balancer name which can be changed (see also [the documentation on load-balancer ID annotations](/docs/getting-started.md#load-balancer-id-annotations)).

Other than that, the only safe place to make load-balancer configuration changes is through the Service object.

### DO load-balancer entry port restrictions

For technical reasons, the ports 50053, 50054, and 50055 cannot be used as load-balancer entry ports (i.e., the port that the load-balancer listens on for requests). Trying to use one of the affected ports as a service port causes a _422 entry port is invalid_ HTTP error response to be returned by the DO API (and surfaced as a Kubernetes event).

The solution is to change the service port to a different, non-conflicting one.

## Development

### Basics

* Go: min `v1.17.x`

This project uses [Go modules](https://github.com/golang/go/wiki/Modules) for dependency management and employs vendoring. Please ensure to run `make vendor` after any dependency modifications.

After making your code changes, run the tests and CI checks:

```bash
make ci
```

### Run Locally

If you want to run `digitalocean-cloud-controller-manager` locally against a
particular cluster, keep your kubeconfig ready and start the binary in the main
package-hosted directory like this:

```bash
cd cloud-controller-manager/cmd/digitalocean-cloud-controller-manager
REGION=fra1 DO_ACCESS_TOKEN=your_access_token go run main.go \
  --kubeconfig <path to your kubeconfig file>                     \
  --leader-elect=false --v=5 --cloud-provider=digitalocean
```

The `REGION` environment variable takes a valid DigitalOcean region.
It can be set to keep `digitalocean-cloud-controller-manager` from trying to access
the DigitalOcean metadata service which is only available on droplets.
If the REGION variable is set, then the DO Regions service will be used to validate the specified region.
It can also be set for local development purposes. Overall,
which region you choose should not matter a lot as long as you pick one.

You might also need to provide your DigitalOcean access token in
`DO_ACCESS_TOKEN` environment variable. The token does not need to be valid for
the cloud controller to start, but in that case, you will not be able to
validate integration with DigitalOcean API.

Please note that if you use a Kubernetes cluster created on DigitalOcean, there
will be a cloud controller manager running in the cluster already, so your local
one will compete for API access with it.

### Optional features

#### Add Public Access Firewall

You can have `digitalocean-cloud-controller-manager` manage a DigitalOcean Firewall
that will dynamically adjust rules for accessing NodePorts: once a Service of type
`NodePort` is created, the firewall controller will update the firewall to public
allow access to just that NodePort. Likewise, access is automatically retracted
if the Service gets deleted or changed to a different type.

Example invocation:

```bash
cd cloud-controller-manager/cmd/digitalocean-cloud-controller-manager
DO_ACCESS_TOKEN=<your_access_token>                           \
PUBLIC_ACCESS_FIREWALL_NAME=firewall_name                     \
PUBLIC_ACCESS_FIREWALL_TAGS=worker-droplet                    \
digitalocean-cloud-controller-manager                         \
  --kubeconfig <path to your kubeconfig file>                 \
  --leader-elect=false --v=5 --cloud-provider=digitalocean
```

The `PUBLIC_ACCESS_FIREWALL_NAME` environment variable defines the name of the
firewall. The firewall is created if no firewall by that name is found.

The `PUBLIC_ACCESS_FIREWALL_TAGS` environment variable refers to the tags
associated with the droplets that the firewall should apply to. Usually, this
is a tag attached to the worker node droplets. Multiple tags are applied in
a logical OR fashion.

In some cases, firewall management for a particular Service may not be
desirable. One example is that a NodePort is supposed to be accessible over the
VPC only. In such cases, the Service annotation
`kubernetes.digitalocean.com/firewall-managed` can be used to selectively
exclude a given Service from firewall management. If set to `"false"`, no
inbound rules will be created for the Service, effectively disabling public
access to the NodePort. (Note the quotes that must be included with "boolean"
annotation values.) The default behavior applies if the annotation is omitted,
is set to `"true`", or contains an invalid value.

No firewall is managed if the environment variables are missing or left empty.
Once the firewall is created, no public access other than to the NodePorts is
allowed. Users should create additional firewalls to further extend access.

#### Expose Prometheus Metrics

If you are interested in exposing Prometheus metrics, you can pass in a metrics
endpoint that will expose them. The command will look similar to this:

```bash
cd cloud-controller-manager/cmd/digitalocean-cloud-controller-manager
DO_ACCESS_TOKEN=your_access_token                  \
METRICS_ADDR=<host>:<port>                         \
digitalocean-cloud-controller-manager              \
  --kubeconfig <path to your kubeconfig file>      \
  --leader-elect=false --v=5 --cloud-provider=digitalocean
```

The `METRICS_ADDR` environment variable takes a valid endpoint that you'd
like to use to serve your Prometheus metrics. To be valid it should be in the
form `<host>:<port>`.

After you have started up `digitalocean-cloud-controller-manager`, run the
following curl command to view the Prometheus metrics output:

```bash
curl <host>:<port>/metrics
```

#### Admission Server

The admission server is an optional component aiming at reducing bad config changes for DO managed objects (LBs, etc).
If you want to know more about it, read the [docs](./docs/admission-server.md).

### DO API rate limiting

DO API usage is subject to [certain rate limits](https://docs.digitalocean.com/reference/api/api-reference/#section/Introduction/Rate-Limit). In order to protect against running out of quota for extremely heavy regular usage or pathological cases (e.g., bugs or API thrashing due to an interfering third-party controller), a custom rate limit can be configured via the `DO_API_RATE_LIMIT_QPS` environment variable. It accepts a float value, e.g., `DO_API_RATE_LIMIT_QPS=3.5` to restrict API usage to 3.5 queries per second.    

### Run Containerized

If you want to test your changes in a containerized environment, create a new
image with the version set to `dev`:

```bash
VERSION=dev make publish
```

This will create a binary with version `dev` and docker image pushed to
`digitalocean/digitalocean-cloud-controller-manager:dev`.

## Release a new version

### Update Go and dependencies 
1. Update Go dependencies
   ```shell
   go get -u ./...
   go mod tidy
   go mod vendor
   ```
2. [Update Go version to latest GA version](./go.mod)

### Github Action (preferred)

To create the docker image and generate the manifests, go to the actions page on GitHub and click "Run Workflow". 
Specify the GitHub `<tag>` that you want to create, making sure it is prefixed with a `v`.
Running the workflow also requires that you temporarily turn off "Require a pull request before merging" setting in the master [branch protection rules settings](https://github.com/digitalocean/digitalocean-cloud-controller-manager/settings/branches). **Don't forget to turn it back on once the release is done!**

The workflow does the following:

* Runs `make bump-version` with `<tag>`
* Creates the ccm related manifests file as `<tag>.yaml`
* Commits the manifest file under `releases/` directory in the repo
* Creates release and tags the new commit with the `<tag>` specified when workflow is triggered
* Logs in with dockerhub credentials specified as secrets
* Builds the docker image `digitalocean/digitalocean-cloud-controller-manager:<tag>`
* Pushes `digitalocean/digitalocean-cloud-controller-manager:<tag>` to dockerhub

### Manual (deprecated)

NOTE: this workflow is deprecated, please prefer the Github Action workflow described above.

To manually release a new version, first bump the version:

```bash
make NEW_VERSION=v1.0.0 bump-version
```

Make sure everything looks good. Create a new branch with all changes:

```bash
git checkout -b release-<new version> origin/master
git commit -a -v
git push origin release-<new version>
```

After it's merged to master, tag the commit and push it:

```bash
git checkout master
git pull
git tag <new version>
git push origin <new version>
```

Finally, [create a Github
release](https://github.com/digitalocean/digitalocean-cloud-controller-manager/releases/new) from
master with the new version and publish it:

```bash
make publish
```

This will compile a binary containing the new version bundled in a docker image pushed to
`digitalocean/digitalocean-cloud-controller-manager:<new version>`

## Contributing

At DigitalOcean we value and love our community! If you have any issues or would like to contribute, feel free to open an issue/PR and cc any of the maintainers below.
