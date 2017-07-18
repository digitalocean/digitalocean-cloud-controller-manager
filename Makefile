.PHONY: clean compile build push

VERSION ?= v0.6
REGISTRY ?= digitalocean

all: clean compile build push

clean:
	rm -f digitalocean-cloud-controller-manager

compile:
	GOOS=linux go build .

build:
	docker build -t $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION) .

push:
	docker push $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION)

