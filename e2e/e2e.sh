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

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
readonly SCRIPTS_DIR="${SCRIPT_DIR}/scripts"

if [[ $# -gt 1 ]]; then
  echo "usage: $(basename "$0") [test filter]"  >&2
  exit 1
fi

RUN="${E2E_RUN_FILTER:-}"
if [[ "${RUN}" ]]; then
  RUN="-run ${RUN}"
fi

echo "==> installing dependencies..."
"${SCRIPTS_DIR}/install_deps.sh"

(
  cd "${SCRIPT_DIR}"
  echo "==> running E2E tests..."
  # shellcheck disable=SC2086
  GO111MODULE=on go test -mod=vendor -v -timeout 1h -count 1 -tags integration ${RUN} "./..."
)
