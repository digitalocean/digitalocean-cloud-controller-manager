# Copyright 2023 DigitalOcean
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the binary
FROM golang:1.19 as builder

WORKDIR /usr/src/app

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -a -o /digitalocean-webhook-server ./cloud-controller-manager/cmd/digitalocean-webhook-server/...

# Use distroless as minimal base image to package the binary
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /digitalocean-webhook-server /digitalocean-webhook-server

ENTRYPOINT ["/digitalocean-webhook-server"]

