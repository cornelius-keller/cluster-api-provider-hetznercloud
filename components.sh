#!/bin/sh
kubectl get secret $1-kubeconfig -o json | jq -r .data.value | base64 -d > kc.yaml
kubectl --kubeconfig=./kc.yaml   apply -f https://docs.projectcalico.org/v3.12/manifests/calico.yaml
kubectl --kubeconfig=./kc.yaml -n kube-system create secret generic hcloud --from-literal=token=$HC_ACCESS_TOKEN
kubectl --kubeconfig=./kc.yaml apply -f  https://raw.githubusercontent.com/hetznercloud/hcloud-cloud-controller-manager/master/deploy/v1.5.1.yaml


