FROM ubuntu:16.04

RUN apt-get update -y && apt-get install -y ca-certificates

ADD digitalocean-cloud-controller-manager /bin/

CMD ["/bin/digitalocean-cloud-controller-manager"]
