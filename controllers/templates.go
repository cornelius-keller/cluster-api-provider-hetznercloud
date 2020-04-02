package controllers

type Template struct {
	Template    string
	Permissions string
	Path        string
}

var installScriptTemplate = Template{
	Template: `#!/bin/sh
apt-get update && apt-get install -y wget docker.io conntrack
cd /
wget https://github.com/kubernetes/kubernetes/releases/download/{{ .Version }}/kubernetes.tar.gz
tar -xzf  kubernetes.tar.gz
KUBERNETES_SKIP_CONFIRM="true" /kubernetes/cluster/get-kube-binaries.sh -y
tar -xzf /kubernetes/server/kubernetes-server-linux-amd64.tar.gz

find /kubernetes/server/bin/  -type f -executable  -exec  ln -vs "{}" /usr/bin/ ';'`,
	Permissions: "0744",
	Path:        "/tmp/install_k8s.sh"}

var systemdUnit = Template{Template: `[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=http://kubernetes.io/docs/

[Service]
ExecStart=/usr/bin/kubelet
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target`,
	Permissions: "0644",
	Path:        "/etc/systemd/system/kubelet.service",
}

var systemdDropIN = Template{
	Template: `[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
# This is a file that "kubeadm init" and "kubeadm join" generate at runtime, populating the KUBELET_KUBEADM_ARGS variable dynamically
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
# This is a file that the user can use for overrides of the kubelet args as a last resort. Preferably,
#the user should use the .NodeRegistration.KubeletExtraArgs object in the configuration files instead.
# KUBELET_EXTRA_ARGS should be sourced from this file.
EnvironmentFile=-/etc/default/kubelet
ExecStart=
ExecStart=/usr/bin/kubelet $KUBELET_KUBECONFIG_ARGS $KUBELET_CONFIG_ARGS $KUBELET_KUBEADM_ARGS $KUBELET_EXTRA_ARGS`,
	Permissions: "0644",
	Path:        "/etc/systemd/system/kubelet.service.d/10-kubeadm.conf",
}

var addIpTemplate = Template{
	Template:    `sudo ip addr add {{ .Ip }} dev eth0`,
	Path:        "/tmp/set_ip.sh",
	Permissions: "0744",
}

var cloudConfigTemplates = []Template{installScriptTemplate, systemdUnit, systemdDropIN, addIpTemplate}
