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

VERSION ?= v0.1.5
REGISTRY ?= digitalocean

all: clean ci compile build push

.PHONY: clean
clean:
	GOOS=linux go clean -i -x ./...

.PHONY: compile
compile:
	CGO_ENABLED=0 GOOS=linux go build

.PHONY: build
build:
	docker build -t $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION) .

.PHONY: push
push:
	docker push $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION)

.PHONY: ci
ci: check-headers gofmt govet golint test

.PHONY: govet
govet:
	go vet $(shell go list ./... | grep -v vendor)

.PHONY: golint
golint:
	golint $(shell go list ./... | grep -v vendor)

.PHONY: gofmt
gofmt: # run in script cause gofmt will exit 0 even if files need formatting
	ci/gofmt.sh

.PHONY: test
test:
	go test $(shell go list ./... | grep -v vendor)

.PHONY: check-headers
check-headers:
	./ci/headers-*.sh
