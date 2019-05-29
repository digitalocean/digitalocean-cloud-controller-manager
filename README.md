# Kubernetes Cloud Controller Manager for DigitalOcean

[![Build Status](https://travis-ci.org/digitalocean/digitalocean-cloud-controller-manager.svg?branch=master)](https://travis-ci.org/digitalocean/digitalocean-cloud-controller-manager) [![Report Card](https://goreportcard.com/badge/github.com/digitalocean/digitalocean-cloud-controller-manager)](https://goreportcard.com/report/github.com/digitalocean/digitalocean-cloud-controller-manager)

`digitalocean-cloud-controller-manager` is the Kubernetes cloud controller manager implementation for DigitalOcean. Read more about cloud controller managers [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/). Running `digitalocean-cloud-controller-manager` allows you to leverage many of the cloud provider features offered by DigitalOcean on your kubernetes clusters.

## Releases

Cloud Controller Manager follows [semantic versioning](https://semver.org/).
The current version is: **`v0.1.14`**. This means that the project is still
under active development and may not be production ready. The plugin will be
bumped to **`v1.0.0`** once the [DigitalOcean Kubernetes
product](https://www.digitalocean.com/products/kubernetes/) is released and
will continue following the rules below:

* Bug fixes will be released as a `PATCH` update.
* New features will be released as a `MINOR` update.
* Significant breaking changes makes a `MAJOR` update.

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

## Development

Requirements:

* Go: min `v1.11.x`

This project uses [Go modules](https://github.com/golang/go/wiki/Modules) for dependency management and employs vendoring. Please ensure to run `make vendor` after any dependency modifications.

After making your code changes, run the tests and CI checks:

```bash
make ci
```

If you want to test your changes, create a new image with the version set to `dev`:

```bash
VERSION=dev make publish
```

This will create a binary with version `dev` and docker image pushed to
`digitalocean/digitalocean-cloud-controller-manager:dev`

### Release a new version

To release a new version first bump the version:

```bash
make bump-version
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
git push --tags
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
