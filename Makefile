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

ifeq ($(strip $(shell git status --porcelain 2>/dev/null)),)
  GIT_TREE_STATE=clean
else
  GIT_TREE_STATE=dirty
endif

## Bump the version in the version file. Set BUMP to [ patch | major | minor ]
BUMP := patch

COMMIT ?= $(shell git rev-parse HEAD)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
VERSION ?= $(shell cat VERSION)
REGISTRY ?= digitalocean
GOVERSION ?= 1.10.3

LDFLAGS ?= -X github.com/digitalocean/digitalocean-cloud-controller-manager/vendor/k8s.io/kubernetes/pkg/version.gitVersion=$(VERSION) -X github.com/digitalocean/digitalocean-cloud-controller-manager/vendor/k8s.io/kubernetes/pkg/version.gitCommit=$(COMMIT) -X github.com/digitalocean/digitalocean-cloud-controller-manager/vendor/k8s.io/kubernetes/pkg/version.gitTreeState=$(GIT_TREE_STATE)
PKG ?= github.com/digitalocean/digitalocean-cloud-controller-manager/cloud-controller-manager/cmd/digitalocean-cloud-controller-manager

all: test

publish: clean ci compile build push

ci: check-headers gofmt govet golint test

.PHONY: e2e
e2e:
	@./e2e/e2e.sh

.PHONY: bump-version
bump-version: 
	@go get -u github.com/jessfraz/junk/sembump # update sembump tool
	$(eval NEW_VERSION = $(shell sembump --kind $(BUMP) $(VERSION)))
	@echo "Bumping VERSION from $(VERSION) to $(NEW_VERSION)"
	@echo $(NEW_VERSION) > VERSION
	@cp releases/${VERSION}.yml releases/${NEW_VERSION}.yml
	@sed -i'' -e 's/${VERSION}/${NEW_VERSION}/g' releases/${NEW_VERSION}.yml
	@sed -i'' -e 's/${VERSION}/${NEW_VERSION}/g' docs/getting-started.md
	@sed -i'' -e 's/${VERSION}/${NEW_VERSION}/g' README.md
	@rm README.md-e docs/getting-started.md-e releases/${NEW_VERSION}.yml-e

.PHONY: clean
clean:
	@echo "==> Cleaning releases"
	@GOOS=linux go clean -i -x ./...

.PHONY: compile
compile:
	@echo "==> Building the project"
	@docker run -v $(PWD):/go/src/github.com/digitalocean/digitalocean-cloud-controller-manager \
	  -w /go/src/github.com/digitalocean/digitalocean-cloud-controller-manager \
	  -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=0 golang:$(GOVERSION) \
	  go build -ldflags "$(LDFLAGS)" ${PKG}

.PHONY: build
build:
	@echo "==> Building the docker image"
	@docker build -t $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION) -f cloud-controller-manager/cmd/digitalocean-cloud-controller-manager/Dockerfile .


.PHONY: push
push:

ifeq ($(shell [[ $(BRANCH) != "master" && $(VERSION) != "dev" ]] && echo true ),true)
	@echo "ERROR: Publishing image with a SEMVER version '$(VERSION)' is only allowed from master"
else
	@echo "==> Publishing $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION)"
	@docker push $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION)
	@echo "==> Your image is now available at $(REGISTRY)/digitalocean-cloud-controller-manager:$(VERSION)"
endif


.PHONY: govet
govet:
	@go vet $(shell go list ./... | grep -v vendor)

.PHONY: golint
golint:
	@golint $(shell go list ./... | grep -v vendor)

.PHONY: gofmt
gofmt: # run in script cause gofmt will exit 0 even if files need formatting
	@ci/gofmt.sh

.PHONY: gofmt
test:
	@echo "==> Testing all packages"
	@go test $(shell go list ./... | grep -v vendor)

.PHONY: check-headers
check-headers:
	@./ci/headers-*.sh

