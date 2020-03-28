module github.com/cornelius-keller/cluster-api-provider-hetznercloud

go 1.13

require (
	github.com/coreos/etcd v3.3.15+incompatible // indirect
	github.com/go-logr/logr v0.1.0
	github.com/hetznercloud/hcloud-go v1.17.0
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/pkg/errors v0.9.1
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/cluster-api v0.3.3
	sigs.k8s.io/controller-runtime v0.5.2
	sigs.k8s.io/testing_frameworks v0.1.2 // indirect
)
