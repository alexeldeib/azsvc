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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	azurev1alpha1 "github.com/alexeldeib/azsvc/api/v1alpha1"
	"github.com/alexeldeib/azsvc/pkg/constants"
	"github.com/alexeldeib/azsvc/pkg/finalizer"
	"github.com/alexeldeib/azsvc/pkg/services/agentpools"
	"github.com/alexeldeib/azsvc/pkg/services/managedclusters"
)

// ManagedClusterReconciler reconciles a ManagedCluster object
type ManagedClusterReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	ClusterService *managedclusters.Service
	PoolService    *agentpools.Service
}

// +kubebuilder:rbac:groups=azure.alexeldeib.xyz,resources=managedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=azure.alexeldeib.xyz,resources=managedclusters/status,verbs=get;update;patch

func (r *ManagedClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcluster", req.NamespacedName)

	obj := &azurev1alpha1.ManagedCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("checking deletion timestamp")
	if obj.GetDeletionTimestamp().IsZero() {
		log.Info("will try to add finalizer")
		if !finalizer.Has(obj, constants.Finalizer) {
			finalizer.Add(obj, constants.Finalizer)
			log.Info("added finalizer")
			return ctrl.Result{}, r.Update(ctx, obj)
		}
	} else {
		log.Info("checking for finalizer")
		if finalizer.Has(obj, constants.Finalizer) {
			log.Info("finalizer present, invoking actuator deletion")
			if err := r.ClusterService.Delete(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
			finalizer.Remove(obj, constants.Finalizer)
			return ctrl.Result{}, r.Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	// TODO(ace): use real creds
	if err := r.ClusterService.Ensure(ctx, log, obj, nil); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&azurev1alpha1.ManagedCluster{}).
		Complete(r)
}
