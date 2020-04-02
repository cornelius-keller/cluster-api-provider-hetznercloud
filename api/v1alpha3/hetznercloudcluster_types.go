/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// MachineFinalizer allows reconciliation loop to clean up resources associated with HetznerCloudMachine before
	// removing it from the apiserver.
	ClusterFinalizer = "hetznercloudcluster.infrastructure.cluster.x-k8s.io"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HetznerCloudClusterSpec defines the desired state of HetznerCloudCluster
type HetznerCloudClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of HetznerCloudCluster. Edit HetznerCloudCluster_types.go to remove/update
	Datacenter string `json:"datacenter"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving.
	Host string `json:"host"`

	// Port is the port on which the API server is serving.
	Port int `json:"port"`
}

// HetznerCloudClusterStatus defines the observed state of HetznerCloudCluster
type HetznerCloudClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready denotes that the hetzner cluster infrastructure is ready
	Ready        bool `json:"ready"`
	FloatingIpId int  `json:"floatingIpId"`
}

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true

// HetznerCloudCluster is the Schema for the hetznercloudclusters API
type HetznerCloudCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HetznerCloudClusterSpec   `json:"spec,omitempty"`
	Status HetznerCloudClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HetznerCloudClusterList contains a list of HetznerCloudCluster
type HetznerCloudClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HetznerCloudCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HetznerCloudCluster{}, &HetznerCloudClusterList{})
}
