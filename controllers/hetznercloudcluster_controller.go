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

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrastructurev1alpha3 "github.com/cornelius-keller/cluster-api-provider-hetznercloud/api/v1alpha3"
)

// HetznerCloudClusterReconciler reconciles a HetznerCloudCluster object
type HetznerCloudClusterReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	HClient *hcloud.Client
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznercloudclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=hetznercloudclusters/status,verbs=get;update;patch

func (r *HetznerCloudClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("hetznercloudcluster", req.NamespacedName)

	var hcluster infrastructurev1alpha3.HetznerCloudCluster
	if err := r.Get(ctx, req.NamespacedName, &hcluster); err != nil {
		// 	import apierrors "k8s.io/apimachinery/pkg/api/errors"
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if hcluster.Spec.ControlPlaneEndpoint.Host == "" {
		location, _, err := r.HClient.Location.GetByName(ctx, hcluster.Spec.Datacenter)
		if err != nil {
			return ctrl.Result{}, err
		}
		floatingip, _, err := r.HClient.FloatingIP.Create(ctx, hcloud.FloatingIPCreateOpts{
			HomeLocation: location,
			Type:         hcloud.FloatingIPTypeIPv4,
		})

		// patch from sigs.k8s.io/cluster-api/util/patch

		helper, err := patch.NewHelper(&hcluster, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}
		hcluster.Spec.ControlPlaneEndpoint = infrastructurev1alpha3.APIEndpoint{

			Host: floatingip.FloatingIP.IP.String(),
			Port: 6443,
		}

		hcluster.Status.Ready = true
		hcluster.Status.FloatingIpId = floatingip.FloatingIP.ID
		controllerutil.AddFinalizer(&hcluster, infrastructurev1alpha3.ClusterFinalizer)

		if err := helper.Patch(ctx, &hcluster); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "couldn't patch cluster %q", hcluster.Name)
		}

	} else if hcluster.Spec.ControlPlaneEndpoint.Host != "" && hcluster.Status.Ready == false {
		// support pivoting
		floatingips, err := r.HClient.FloatingIP.All(ctx)

		if err != nil {
			return ctrl.Result{}, err
		}

		for _, ip := range floatingips {
			if ip.IP.String() == hcluster.Spec.ControlPlaneEndpoint.Host {
				helper, err := patch.NewHelper(&hcluster, r.Client)
				if err != nil {
					return ctrl.Result{}, err
				}
				hcluster.Status.Ready = true
				hcluster.Status.FloatingIpId = ip.ID
				controllerutil.AddFinalizer(&hcluster, infrastructurev1alpha3.ClusterFinalizer)

				if err := helper.Patch(ctx, &hcluster); err != nil {
					return ctrl.Result{}, errors.Wrapf(err, "couldn't patch cluster %q", hcluster.Name)
				}
			}
		}
		if hcluster.Status.Ready == false {
			return ctrl.Result{}, errors.New("floating ip not found but present in spec")
		}

	}

	// handle deleteion
	if !hcluster.ObjectMeta.DeletionTimestamp.IsZero() {

		helper, err := patch.NewHelper(&hcluster, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.UnassignFIP(hcluster.Status.FloatingIpId)
		if err != nil {
			return ctrl.Result{}, err
		}

		controllerutil.RemoveFinalizer(&hcluster, infrastructurev1alpha3.ClusterFinalizer)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := helper.Patch(ctx, &hcluster); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "couldn't patch cluster %q", hcluster.Name)
		}

	}

	return ctrl.Result{}, nil
}

func (r *HetznerCloudClusterReconciler) UnassignFIP(FloatingIPID int) error {
	ctx := context.Background()
	fip, _, err := r.HClient.FloatingIP.GetByID(ctx, FloatingIPID)

	if err != nil {
		return err
	}

	if fip.Server != nil {
		_, _, err = r.HClient.FloatingIP.Unassign(ctx, &hcloud.FloatingIP{ID: FloatingIPID})
		if err != nil {
			r.Log.Error(err, "Error unassigning foating ip")
			return err
		}
	}

	_, err = r.HClient.FloatingIP.Delete(ctx, &hcloud.FloatingIP{ID: FloatingIPID})
	if err != nil {
		r.Log.Error(err, "Error deleting foating ip")
		return err
	}
	return nil

}

func (r *HetznerCloudClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha3.HetznerCloudCluster{}).
		Complete(r)
}
