#!/bin/sh
kubectl get secret $1-kubeconfig -o json | jq -r .data.value | base64 -d > kc.yaml
