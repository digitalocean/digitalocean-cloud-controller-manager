#!/bin/bash

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

if [[ -z $DIGITALOCEAN_ACCESS_TOKEN ]]; then
  echo "DIGITALOCEAN_ACCESS_TOKEN is empty, this must be set to your DigitalOcean access token..."
  exit 1
fi

ENCODED_ACCESS_TOKEN=$(echo -n "$DIGITALOCEAN_ACCESS_TOKEN" | base64)

GENERATED_SECRET=./generated-secret.yml
trap "{ rm $GENERATED_SECRET; }" EXIT

cat > $GENERATED_SECRET <<EOF
---
apiVersion: v1
kind: Secret
metadata:
  name: digitalocean
  namespace: kube-system
data:
  access-token: $ENCODED_ACCESS_TOKEN
EOF

echo "Generated secret:"
cat $GENERATED_SECRET

read -p "Create the following secret to your default kubectl context? [y/N]: " -r

if [[ ! $REPLY =~ ^[y]$ ]]
then
	exit 0
fi

echo "Creating Kubernetes Secret for DigitalOcean"
kubectl apply -f $GENERATED_SECRET
