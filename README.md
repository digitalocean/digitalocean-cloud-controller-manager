# Kubernetes Cloud Controller Manager for DigitalOcean
[![Build Status](https://travis-ci.org/digitalocean/digitalocean-cloud-controller-manager.svg?branch=master)](https://travis-ci.org/digitalocean/digitalocean-cloud-controller-manager) [![Report Card](https://goreportcard.com/badge/github.com/digitalocean/digitalocean-cloud-controller-manager)](https://goreportcard.com/report/github.com/digitalocean/digitalocean-cloud-controller-manager)

`digitalocean-cloud-controller-manager` is the Kubernetes cloud controller manager implementation for DigitalOcean. Read more about cloud controller managers [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/). Running `digitalocean-cloud-controller-manager` allows you to leverage many of the cloud provider features offered by DigitalOcean on your kubernetes clusters.

**WARNING**: this project is a work in progress and may not be production ready.

## Getting Started!

Learn more about running DigitalOcean cloud controller manager [here](docs/getting-started.md)!

## Examples

Here are some examples of how you could leverage `digitalocean-cloud-controller-manager`:
* [loadbalancers](examples/loadbalancers/)
* [node labels and addresses](examples/nodes/)

## Contributing
At DigitalOcean we value and love our community! If you have any issues or would like to contribute, feel free to open an issue/PR and cc any of the maintainers below.

### Maintainers
* Andrew Sy Kim - @andrewsykim
* Billie Cleek - @bhcleek
