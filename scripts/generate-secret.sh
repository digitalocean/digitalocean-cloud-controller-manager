#!/bin/bash

if [[ -z $DO_ACCESS_TOKEN ]]; then
  echo "DO_ACCESS_TOKEN is empty, this must be set to your DigitalOcean access token..."
  exit 1
fi

ENCODED_ACCESS_TOKEN=$(echo -n "$DO_ACCESS_TOKEN" | base64)

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
