# end-to-end testing

This package holds end-to-end tests to verify CCM functionality for a set of supported Kubernetes versions.

It may be run either locally or on the CI.

## usage

The easiest way to run the end-to-end test is to execute `docker.sh test`. It requires the following environment variables to be set or available in a `.env` file:

- `DIGITALOCEAN_ACCESS_TOKEN`: the DigitalOcean access token
- `KOPS_REGION`: the DigitalOcean region (e.g., _sfo2_)
- `S3_ENDPOINT`: the Spaces endpoint; should consist of the `KOPS_REGION` followed by the official Spaces API host (e.g., _sfo2.digitaloceanspaces.com_)
- `S3_ACCESS_KEY_ID`: the Spaces access key ID
- `S3_SECRET_ACCESS_KEY`: the Spaces secret access key ID
- `KOPS_CLUSTER_NAME`: The cluster name; needs to be managed by the DigitalOcean nameserver (e.g., _k8stest.mydomain.com_)

The following environment parameters are optional:

- `SSH_PUBLIC_KEYFILE`: path to the public SSH key matching the private key configured on DigitalOcean (default: `$HOME/.ssh/id_rsa.pub`)

`e2e.sh` invokes the end-to-end tests which in turn invoke `setup_cluster.sh` and `destroy_cluster.sh` to create clusters before the test executes and tears them down again on completion of a test, respectively. The scripts may also be called manually, which then needs the `KOPS_STATE_STORE` environment variable to be set explicitly and point to the Spaces bucket name as in `do://myuniquespace`.

`e2e.sh` reads the `E2E_RUN_FILTER` environment variable and passes any non-empty content to `go test`'s `-run` flag to filter for specific tests.

## resource cleanup

The end-to-end tests clean up resources used on the DigitalOcean cloud after themselves on completion (either successful or erroneous). In the case that teardown does not complete for whatever reason (say, because of a crash of the tests or resources being in an irreparable state), the `docker.sh clean <ID> <NAME>` script can be used to remove the resources explicitly.

See the script's header for details.
