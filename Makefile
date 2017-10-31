# Copyright 2017 DigitalOcean
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

VERSION ?= v0.1.2
REGISTRY ?= digitalocean

all: clean compile build push

.PHONY: clean compile build push test govet golint gofmt

clean:
	rm -f digitalocean-cloud-controller-manager

compile:
	GOOS=linux go build .

build:
	docker build -t $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION) .

push:
	docker push $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION)

ci: check-headers govet gofmt test

govet:
	go vet $(shell go list ./... | grep -v vendor)

golint:
	golint $(shell go list ./... | grep -v vendor)

gofmt: # run in script cause gofmt will exit 0 even if files need formatting
	ci/gofmt.sh

test:
	go test $(shell go list ./... | grep -v vendor)

check-headers:
	./ci/headers-*.sh
