#!/usr/bin/env bash
set -e

KUBERNETES_VERSION=${KUBERNETES_VERSION:?}

deps=()

while read -ra LINE
do
  depname="${LINE[0]}"
  if [ "$depname" == "k8s.io/kubernetes" ] ;then
    deps+=("-replace $depname=$depname@v$KUBERNETES_VERSION")
  else
    deps+=("-replace $depname=$depname@kubernetes-$KUBERNETES_VERSION")
  fi
done < <(curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/v$KUBERNETES_VERSION/go.mod" \
  | grep -E '^[[:space:]]*k8s.io.* v0.0.0$')

unset GOROOT GOPATH
export GO111MODULE=on

set -x
# shellcheck disable=SC2086
go mod edit ${deps[*]}
go mod tidy
go mod vendor
set +x

sed -i -e "s/^KUBERNETES_VERSION.*/KUBERNETES_VERSION ?= $KUBERNETES_VERSION/" Makefile
