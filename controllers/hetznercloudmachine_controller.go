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

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrastructurev1alpha3 "github.com/cornelius-keller/cluster-api-provider-hetznercloud/api/v1alpha3"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
)

// HetznerCloudMachineReconciler reconciles a HetznerCloudMachine object
type HetznerCloudMachineReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	HClient *hcloud.Client
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznercloudmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznercloudmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *HetznerCloudMachineReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("hetznercloudmachine", req.NamespacedName)

	// Fetch the DockerMachine instance.
	hetznerMachine := &infrastructurev1alpha3.HetznerCloudMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, hetznerMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, hetznerMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Waiting for Machine Controller to set OwnerRef on HetznerCloudMachine")
		return ctrl.Result{Requeue: true, RequeueAfter: 10}, nil
	}

	// Handle deleted machines
	if !hetznerMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machine, hetznerMachine)
	}

	log = log.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("HetznerCloudMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterLabelName))
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for HetznerCloudCluster Controller to create cluster infrastructure")
		return ctrl.Result{Requeue: true, RequeueAfter: 10}, nil
	}

	// Fetch the HetznerCloudCluster Cluster.
	hetznerCluster := &infrastructurev1alpha3.HetznerCloudCluster{}
	hetznerClusterName := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		log.Info(fmt.Sprintf("HetznerCloudCluster '%s' is  not available yet in namespace '%s'", hetznerClusterName.Name, hetznerClusterName.Namespace))
		return ctrl.Result{Requeue: true}, nil
	}

	log = log.WithValues("docker-cluster", hetznerCluster.Name)

	return r.reconcileNormal(ctx, machine, hetznerMachine, hetznerCluster, cluster, log)

}

func (r *HetznerCloudMachineReconciler) reconcileNormal(ctx context.Context, machine *clusterv1.Machine,
	hetznerMachine *infrastructurev1alpha3.HetznerCloudMachine, hetznerCluster *infrastructurev1alpha3.HetznerCloudCluster, cluster *clusterv1.Cluster,
	log logr.Logger) (ctrl.Result, error) {

	// if the machine is already provisioned, return
	if hetznerMachine.Status.ProviderId != nil {
		// ensure ready state is set.
		// This is required after move, bacuse status is not moved to the target cluster.
		hetznerMachine.Status.Ready = true
		return ctrl.Result{}, nil
	}

	// get kubeadm bootstrap CR for machine

	bootstrapConfig := &bootstrapv1.KubeadmConfig{}
	bootstrapConfigName := client.ObjectKey{
		Namespace: machine.Spec.Bootstrap.ConfigRef.Namespace,
		Name:      machine.Spec.Bootstrap.ConfigRef.Name,
	}
	if err := r.Client.Get(ctx, bootstrapConfigName, bootstrapConfig); err != nil {
		log.Info(fmt.Sprintf("KubeadmConfig '%s' is  not available yet in namespace '%s'", bootstrapConfigName.Name, bootstrapConfigName.Namespace))
		return ctrl.Result{Requeue: true}, nil
	}

	// inject needed fields
	if len(bootstrapConfig.Spec.Files) == 0 {
		// add install script

		patchHelper, err := patch.NewHelper(bootstrapConfig, r)
		if err != nil {
			return ctrl.Result{}, err
		}

		type Values struct {
			Ip      string
			Version string
		}

		templateValues := Values{Version: *machine.Spec.Version, Ip: cluster.Spec.ControlPlaneEndpoint.Host}
		for _, tmpl := range cloudConfigTemplates {

			t, err := template.New("test").Parse(tmpl.Template)
			if err != nil {
				r.Log.Error(err, "failed to initialize template")
				return ctrl.Result{}, nil
			}
			buffer := new(bytes.Buffer)
			t.Execute(buffer, templateValues)

			bootstrapConfig.Spec.Files = append(bootstrapConfig.Spec.Files,
				bootstrapv1.File{Path: tmpl.Path,
					Permissions: tmpl.Permissions,
					Content:     buffer.String(),
				})
		}

		if util.IsControlPlaneMachine(machine) {
			bootstrapConfig.Spec.PreKubeadmCommands = []string{
				"/tmp/install_k8s.sh",
				"/tmp/set_ip.sh",
			}
		} else {
			bootstrapConfig.Spec.PreKubeadmCommands = []string{
				"/tmp/install_k8s.sh",
			}
		}

		bootstrapConfig.Spec.PostKubeadmCommands = []string{
			"/bin/systemctl daemon reload",
			"/bin/systemctl enable kubelet.service",
			"/bin/systemctl start kubelet.service",
		}

		patchHelper.Patch(ctx, bootstrapConfig)

	}

	// wait until it gets reconsiled and data is created.

	// Make sure bootstrap data is available and populated.
	if machine.Spec.Bootstrap.DataSecretName == nil {
		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return ctrl.Result{Requeue: true}, nil
	}

	if hetznerMachine.Status.ProviderId == nil {
		bootstrapData, err := r.getBootstrapData(ctx, machine)
		if err != nil {
			r.Log.Error(err, "failed to get bootstrap data")
			return ctrl.Result{}, nil
		}

		if !strings.Contains(bootstrapData, "/tmp/install_k8s.sh") {
			r.Log.Info("bootstrap data does not contain needed files yet")
			return ctrl.Result{Requeue: true}, nil
		}

		// create a machine with the bootstrap data

		sshKey, _, err := r.HClient.SSHKey.Get(ctx, "jck@cornelius-pc")
		if err != nil {
			r.Log.Error(err, "failed to get ssh key")
			return ctrl.Result{}, nil
		}

		serverOpts := hcloud.ServerCreateOpts{
			Name: hetznerMachine.Name,
			ServerType: &hcloud.ServerType{
				Name: hetznerMachine.Spec.Type,
			},
			Image: &hcloud.Image{
				Name: "ubuntu-18.04",
			},
			Location: &hcloud.Location{Name: hetznerCluster.Spec.Datacenter},

			UserData: bootstrapData,
			SSHKeys: []*hcloud.SSHKey{
				sshKey,
			},
		}

		server, _, err := r.HClient.Server.Create(ctx, serverOpts)
		if err != nil {
			return ctrl.Result{}, err
		}

		//r.HClient.FloatingIP.GetByID(ctx, hetznerCluster.Status.FloatingIpId)
		r.HClient.FloatingIP.Assign(ctx, &hcloud.FloatingIP{ID: hetznerCluster.Status.FloatingIpId}, server.Server)

		// Initialize the patch helper
		patchHelper, err := patch.NewHelper(hetznerMachine, r)
		if err != nil {
			return ctrl.Result{}, err
		}

		// If the HetznerMachine doesn't have finalizer, add it.
		controllerutil.AddFinalizer(hetznerMachine, infrastructurev1alpha3.MachineFinalizer)

		serverID := strconv.Itoa(server.Server.ID)
		hetznerMachine.Status.ProviderId = &serverID
		hetznerMachine.Status.Ready = true
		if err := patchHelper.Patch(ctx, hetznerMachine); err != nil {
			log.Error(err, "failed to patch HetznerCloudMachine")
		}
	}

	return ctrl.Result{}, nil
}

func (r *HetznerCloudMachineReconciler) reconcileDelete(ctx context.Context, machine *clusterv1.Machine, hetznerMachine *infrastructurev1alpha3.HetznerCloudMachine) (ctrl.Result, error) {
	// Long lived CAPD clusters (unadvised) that are using Machine resources for the control plane machines
	// will have to manually keep the kubeadm config-map on the workload cluster up to date.
	// This is automated when using the KubeadmControlPlane.

	// delete the machine

	serverid, err := strconv.Atoi(*hetznerMachine.Status.ProviderId)
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = r.HClient.Server.Delete(ctx, &hcloud.Server{ID: serverid})
	if err != nil {
		return ctrl.Result{}, err
	}

	if err != nil {
		return ctrl.Result{}, err
	}
	// Machine is deleted so remove the finalizer.
	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(hetznerMachine, r)
	if err != nil {
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(hetznerMachine, infrastructurev1alpha3.MachineFinalizer)
	if err := patchHelper.Patch(ctx, hetznerMachine); err != nil {
		log.Error(err, "failed to patch HetznerCloudMachine")
	}
	return ctrl.Result{}, nil
}

func (r *HetznerCloudMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha3.HetznerCloudMachine{}).
		Complete(r)
}

func (r *HetznerCloudMachineReconciler) getBootstrapData(ctx context.Context, machine *clusterv1.Machine) (string, error) {
	if machine.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	s := &corev1.Secret{}
	key := client.ObjectKey{Namespace: machine.GetNamespace(), Name: *machine.Spec.Bootstrap.DataSecretName}
	if err := r.Client.Get(ctx, key, s); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for DockerMachine %s/%s", machine.GetNamespace(), machine.GetName())
	}

	value, ok := s.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	//return base64.StdEncoding.EncodeToString(value), nil
	return string(value), nil
}
