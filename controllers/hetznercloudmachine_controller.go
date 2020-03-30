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
	"context"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrastructurev1alpha3 "github.com/cornelius-keller/cluster-api-provider-hetznercloud/api/v1alpha3"
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
		log.Info("Waiting for Machine Controller to set OwnerRef on DockerMachine")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("DockerMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterLabelName))
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for DockerCluster Controller to create cluster infrastructure")
		return ctrl.Result{}, nil
	}

	// Fetch the HetznerCloudCluster Cluster.
	hetznerCluster := &infrastructurev1alpha3.HetznerCloudCluster{}
	hetznerClusterName := client.ObjectKey{
		Namespace: hetznerCluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, hetznerClusterName, hetznerCluster); err != nil {
		log.Info("DockerCluster is not available yet")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("docker-cluster", hetznerCluster.Name)

	// Create a helper for managing the docker container hosting the machine.
	/*externalMachine, err := docker.NewMachine(cluster.Name, machine.Name, dockerMachine.Spec.CustomImage, log)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the externalMachine")
	}

	// Create a helper for managing a docker container hosting the loadbalancer.
	// NB. the machine controller has to manage the cluster load balancer because the current implementation of the
	// docker load balancer does not support auto-discovery of control plane nodes, so CAPD should take care of
	// updating the cluster load balancer configuration when control plane machines are added/removed
	externalLoadBalancer, err := docker.NewLoadBalancer(cluster.Name, log)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the externalLoadBalancer")
	} */

	// Initialize the patch helper
	/*
		patchHelper, err := patch.NewHelper(hetznerMachine, r)
		if err != nil {
			return ctrl.Result{}, err
		}
		// Always attempt to Patch the DockerMachine object and status after each reconciliation.
		defer func() {
			if err := patchHelper.Patch(ctx, hetznerMachine); err != nil {
				log.Error(err, "failed to patch DockerMachine")
				if rerr == nil {
					rerr = err
				}
			}
		}()
	*/

	// Handle deleted machines
	if !hetznerMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		// return r.reconcileDelete(ctx, machine, hetznerMachine, heztnerMachine, externalLoadBalancer)
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

	return base64.StdEncoding.EncodeToString(value), nil
}
