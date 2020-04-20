# Cluster API infrastructure provider for hetznercloud

Disclaimer: this is the result of a personal project to learn about cluster api and improve my go coding skills.
This is far from production ready. 

## Description

[Kubernetes Cluster API](https://cluster-api.sigs.k8s.io/) infrastructure provider for [hetznercloud](https://www.hetzner.de/cloud)

### Current Featrures:
- deploys any recent published kubernetes version supporting kubeadm

### What is missing:

- failover ip handling on master failure
- private networks (Currently all ips are public)
- proper defaulting and error handling
- a lot of other things.

Install clusterctl

Configure clusterctl:

$HOME/.cluster-api/clusterctl.yaml

```
providers:
  - name: hetznercloud
    url: https://github.com/cornelius-keller/cluster-api-provider-hetznercloud/releases/v0.3.0/infrastructure_components.yaml
    type: InfrastructureProvider
```

export some settings:
```
export HC_ACCESS_TOKEN=<your hetzner cloud access token>
```

Setup the management cluster (uses current kubecontext)
```
clusterctl init --infrastructure-provider=hetznercloud
```

configure cluster:
```
export HETZNER_DATACENTER=fsn1
export HETZNER_MACHINE_TYPE=cx21
export HETZNER_SSH_KEY=<name of your hetznercloud ssh key>
export POD_CIDR=192.168.0.0/16
export SERVICE_CIDR=192.169.0.0/16
```

create the cluster manifests:
```
clusterctl config cluster my-cluster --kubernetes-version v1.17.3 --control-plane-machine-count=3 --worker-machine-count=3 > my-cluster.yaml
```

create cluster
``` 
kubectl create -f my-cluster.yaml
```

use kc.sh script to create kubeconfig
```
./get_kc.sh my-cluster
```

use the kubeconfig to see if the first master is up:
```
KUBECONFIG=$PWD/kc.yaml kubectl  get nodes
NAME               STATUS     ROLES    AGE     VERSION
my-cluster-bnznd   NotReady   master   5m10s   v1.17.3
```

Once the first master is up but in NotReady state, use components.sh to install missing infrastucture components (out of tree cloud provider, calico)
```
./components.sh my-cluster
configmap/calico-config created
customresourcedefinition.apiextensions.k8s.io/felixconfigurations.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/ipamblocks.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/blockaffinities.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/ipamhandles.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/ipamconfigs.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/bgppeers.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/bgpconfigurations.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/ippools.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/hostendpoints.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/clusterinformations.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/globalnetworkpolicies.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/globalnetworksets.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/networkpolicies.crd.projectcalico.org created
customresourcedefinition.apiextensions.k8s.io/networksets.crd.projectcalico.org created
clusterrole.rbac.authorization.k8s.io/calico-kube-controllers created
clusterrolebinding.rbac.authorization.k8s.io/calico-kube-controllers created
clusterrole.rbac.authorization.k8s.io/calico-node created
clusterrolebinding.rbac.authorization.k8s.io/calico-node created
daemonset.apps/calico-node created
serviceaccount/calico-node created
deployment.apps/calico-kube-controllers created
serviceaccount/calico-kube-controllers created
secret/hcloud created
serviceaccount/cloud-controller-manager created
clusterrolebinding.rbac.authorization.k8s.io/system:cloud-controller-manager created
deployment.apps/hcloud-cloud-controller-manager created
```

After 10 to 15 minutes all nodes should be there:

```
KUBECONFIG=$PWD/kc.yaml kubectl  get nodes
NAME               STATUS   ROLES    AGE     VERSION
my-cluster-bnznd   Ready    master   25m     v1.17.3
my-cluster-fh6qm   Ready    <none>   18m     v1.17.3
my-cluster-jm9zg   Ready    master   5m16s   v1.17.3
my-cluster-l8slt   Ready    <none>   18m     v1.17.3
my-cluster-n8rzj   Ready    master   2m21s   v1.17.3
my-cluster-s7wjm   Ready    <none>   18m     v1.17.3
```

Delete cluster:
```
kubectl delete -f my-cluster.yaml
```

Please check your hetzner cloud accout for unwanted resources or things that are not deleted properly.
I tested carefully and there should not be any remains but this is still in a very early state.










