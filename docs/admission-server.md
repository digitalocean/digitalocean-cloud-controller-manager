# Admission Server

An admission webhook server can be deployed alongside the `digitalocean-cloud-controller-manager` to prevent misconfiguration of DigitalOcean-managed objects.
The server implements [dynamic admission control](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/), validating objects when they are created and updated.

The admission webhook server exposes the following validation webhooks:

* `/lb-service`: Validates LoadBalancer service object creation and update requests.

## Purpose

The admission server checks whether changes to a DO-managed object result in a valid configuration.
In cases where the configuration is deemed invalid, the request is denied, and the user is presented with an error.
This adds a layer of protection for users who could inadvertently produce a flawed set of configurations.

The load balancer validation webhook is especially useful to protect against configuration errors.
Because Kubernetes treats resource updates asynchronously, an inadvertent error in a service annotation could go unnoticed.
A broken set of configurations could fail all further reconciliation attempts, including adjustments to the target droplet set.

## What does it do?

The validation webhook does two things:
1. *Validates that annotations and specs are correct:*
    When validating LoadBalancer services, for instance, it will check that all [annotations](./controllers/services/annotations.md) have expected values.
2. *Query the DO API for additional validations:*
    The admission server may request validation from the DO API. It is important to note that these requests have a validation flag and DO NOT mutate the load balancer instances (a CCM responsibility).

## Requirements

To use the manifests from the [releases](../releases/digitalocean-cloud-controller-manager-admission-server/) directory, you will need:
* [cert-manager](https://cert-manager.io/)
* A `digitalocean` secret containing a DO API token (see the [getting-started-docs](./getting-started.md#manually))

## Installation

The validation webhook is provisioned by default in all DOKS installations.
If you want to install the admission server in your cluster manually, please refer to the [getting-started docs](./getting-started.md#cloud-controller-manager-admission-server).
