#!/usr/bin/env bash

# Copyright 2020 DigitalOcean
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

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
readonly SCRIPT_DIR
# shellcheck source=./utils.sh
source "${SCRIPT_DIR}/utils.sh"

check_envs
check_env 'KOPS_REGION'

if [[ $# -ne 2 ]]; then
  echo "usage: $(basename "$0") <kubernetes version> <number of nodes>" >&2
  exit 1
fi

readonly KUBERNETES_VERSION="$1"
readonly NUM_NODES="$2"

SSH_PUBLIC_KEYFILE=${SSH_PUBLIC_KEYFILE:-${HOME}/.ssh/id_rsa.pub}
if [ ! -f ${SSH_PUBLIC_KEYFILE} ]; then
  echo "${SSH_PUBLIC_KEYFILE} does not exist"
  exit 1
fi
SSH_KEY_FINGERPRINT="$(ssh-keygen -E md5 -lf ${SSH_PUBLIC_KEYFILE} | cut -d " " -f 2 | sed -e "s/^MD5://")"
if ! doctl compute ssh-key get ${SSH_KEY_FINGERPRINT}; then
  echo "${SSH_PUBLIC_KEYFILE} (${SSH_KEY_FINGERPRINT}) does not exist in your DO account"
  exit 1
fi

ensure_deps

echo "creating cluster for version ${KUBERNETES_VERSION} in region ${KOPS_REGION} with ${NUM_NODES} node(s) and SSH public key at ${SSH_PUBLIC_KEYFILE}"
kops create cluster --cloud=digitalocean \
  --kubernetes-version="${KUBERNETES_VERSION}" \
  --name="${KOPS_CLUSTER_NAME}" \
  --zones="${KOPS_REGION}" \
  --ssh-public-key="${SSH_PUBLIC_KEYFILE}" \
  --node-count "${NUM_NODES}" \
  --yes

echo "==> waiting until Kubernetes cluster is ready..."

SECONDS=0
n=0
until [[ $n -ge 300 ]]; do
  if kubectl --request-timeout=5s api-versions > /dev/null; then
    echo "==> Kubernetes cluster is ready (took $((SECONDS / 60)) minutes)"
    exit 0
  fi

  n=$((n+1))
  sleep 5
done

echo "==> timed out waiting for Kubernetes cluster to become ready"
exit 1
