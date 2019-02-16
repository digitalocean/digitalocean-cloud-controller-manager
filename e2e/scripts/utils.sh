#!/usr/bin/env bash

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

set -o errexit
set -o pipefail
set -o nounset

readonly KOPS_VERSION=v1.10.0
declare -rx KOPS_FEATURE_FLAGS=AlphaAllowDO
readonly REQUIRED_ENV_VARS='KOPS_STATE_STORE S3_ENDPOINT S3_ACCESS_KEY_ID S3_SECRET_ACCESS_KEY KOPS_CLUSTER_NAME'

# check_envs verifies that all required environment variables are set.
check_envs() {
  for REQ_ENV_VAR in ${REQUIRED_ENV_VARS}; do
    check_env "${REQ_ENV_VAR}"
  done
}

# check_env verifies that the given environment variable is set.
check_env() {
  declare -r _env="$1"
  if [[ -z "${!_env:-}" ]]; then
    echo "environment variable ${_env} must be set" >&2
    exit 1
  fi
}

ensure_deps() {
  ensure_kubectl
  ensure_kops
}

# ensure_kubectl makes sure kubectl is installed and, if not, installs it when
# running on the CI.
ensure_kubectl() {
  if ! type kubectl > /dev/null 2>&1; then
    if [[ -z ${CI:-} ]]; then
      echo "please install missing dependency: kubectl" >&2
      return 1
    fi
    echo "==> installing kubectl"
    curl --fail --location --remote-name  "https://storage.googleapis.com/kubernetes-release/release/$(curl --fail --silent --show-error https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x ./kubectl
    sudo mv ./kubectl /usr/local/bin/kubectl
  fi
}

# ensure_kops makes sure kops is installed and, if not, installs a specific
# version when running on the CI.
ensure_kops() {
  if ! type kops > /dev/null 2>&1; then
    if [[ -z ${CI:-} ]]; then
      echo "please install missing dependency: kops" >&2
      return 1
    fi
    echo "==> installing kops"
    (
      # kops' Makefile leverages the CI variable; unset it to make sure we
      # build a production release.
      unset CI
      go get -u k8s.io/kops
      cd "${GOPATH}/src/k8s.io/kops"
      git checkout "${KOPS_VERSION}"
      sed -i 's#image: digitalocean/digitalocean-cloud-controller-manager:.*#image: digitalocean/digitalocean-cloud-controller-manager:v0.1.8#' upup/models/cloudup/resources/addons/digitalocean-cloud-controller.addons.k8s.io/k8s-1.8.yaml.template
      make kops
      chmod u+x .build/local/kops
      sudo cp .build/local/kops /usr/local/bin/
    )
  fi
}
