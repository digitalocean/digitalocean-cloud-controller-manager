# Kubernetes Cloud Controller Manager for DigitalOcean
[![Build Status](https://travis-ci.org/digitalocean/digitalocean-cloud-controller-manager.svg?branch=master)](https://travis-ci.org/digitalocean/digitalocean-cloud-controller-manager) [![Report Card](https://goreportcard.com/badge/github.com/digitalocean/digitalocean-cloud-controller-manager)](https://goreportcard.com/report/github.com/digitalocean/digitalocean-cloud-controller-manager)

`digitalocean-cloud-controller-manager` is the Kubernetes cloud controller manager implementation for DigitalOcean. Read more about cloud controller managers [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/). Running `digitalocean-cloud-controller-manager` allows you to leverage many of the cloud provider features offered by DigitalOcean on your kubernetes clusters.


## Releases

Cloud Controller Manager follows [semantic versioning](https://semver.org/).
The current version is: **`v0.1.8`**. This means that the project is still
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


## Getting Started!

Learn more about running DigitalOcean cloud controller manager [here](docs/getting-started.md)!

## Examples

Here are some examples of how you could leverage `digitalocean-cloud-controller-manager`:
* [loadbalancers](docs/controllers/services/examples/)
* [node labels and addresses](docs/controllers/node/examples/)

## Development

Requirements:

* Go: min `v1.10.x`

After making your changes, run the tests and CI checks: 

```
$ make ci
```

If you want to test your changes, create a new image with the version set to `dev`:

```
$ VERSION=dev make publish
```

This will create a binary with version `dev` and docker image pushed to
`digitalocean/digitalocean-cloud-controller-manager:dev`

To release a new version bump first the version:

```
$ make bump-version
```

Make sure everything looks good. Create a new branch with all changes:

```
$ git checkout -b new-release
$ git add .
$ git push origin
```

After it's merged to master, [create a new Github
release](https://github.com/digitalocean/digitalocean-cloud-controller-manager/releases/new) from
master with the version `v0.1.8` and then publish a new docker build:

```
$ git checkout master
$ make publish
```

This will create a binary with version `v0.1.8` and docker image pushed to
`digitalocean/digitalocean-cloud-controller-manager:v0.1.8`

## Contributing
At DigitalOcean we value and love our community! If you have any issues or would like to contribute, feel free to open an issue/PR and cc any of the maintainers below.

### Maintainers
* Andrew Sy Kim - @andrewsykim
* Billie Cleek - @bhcleek
