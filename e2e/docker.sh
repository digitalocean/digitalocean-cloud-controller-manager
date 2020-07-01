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

COMMAND=${1:-}
TARGET_SCRIPT=$(case "${COMMAND}" in
  (test)  echo "e2e.sh";;
  (clean) echo "scripts/cleanup-resources.sh";;
esac)

if [ -z "${TARGET_SCRIPT}" ]; then
    echo "usage: $(basename "$0") test|clean" >&2
    exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_DIR="${SCRIPT_DIR}/.."

(
    cd ${PROJECT_DIR}

    DOCKER_RUN_ARGS=""
    if [ -f e2e/.env ]; then
        DOCKER_RUN_ARGS="--env-file e2e/.env"
    fi

    docker build -t do-ccm-e2e e2e
    docker run \
        --rm -it \
        -v ${PROJECT_DIR}:/app \
        -v ${HOME}/.ssh:/root/.ssh:ro \
        -w /app \
        -e DIGITALOCEAN_ACCESS_TOKEN \
        -e KOPS_REGION \
        -e S3_ENDPOINT \
        -e S3_ACCESS_KEY_ID \
        -e S3_SECRET_ACCESS_KEY \
        -e KOPS_CLUSTER_NAME \
        ${DOCKER_RUN_ARGS} \
        do-ccm-e2e \
        e2e/${TARGET_SCRIPT}
)