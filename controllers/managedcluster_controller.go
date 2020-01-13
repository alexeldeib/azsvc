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
	"io"
	"io/ioutil"

	"github.com/go-logr/logr"
	"github.com/sanity-io/litter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"

	"github.com/alexeldeib/azsvc/api/v1alpha1"
	azurev1alpha1 "github.com/alexeldeib/azsvc/api/v1alpha1"
	"github.com/alexeldeib/azsvc/pkg/constants"
	"github.com/alexeldeib/azsvc/pkg/decoder"
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

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=azure.alexeldeib.xyz,resources=managedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=azure.alexeldeib.xyz,resources=managedclusters/status,verbs=get;update;patch

func (r *ManagedClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcluster", req.NamespacedName)

	obj := &azurev1alpha1.ManagedCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	log.V(2).Info("checking deletion timestamp")
	if obj.GetDeletionTimestamp().IsZero() {
		log.V(2).Info("will try to add finalizer")
		if !finalizer.Has(obj, constants.Finalizer) {
			finalizer.Add(obj, constants.Finalizer)
			log.Info("added finalizer")
			return ctrl.Result{}, r.Update(ctx, obj)
		}
	} else {
		log.V(2).Info("checking for finalizer")
		if finalizer.Has(obj, constants.Finalizer) {
			log.Info("finalizer present, invoking deletion")
			if err := r.ClusterService.Delete(ctx, log, obj); err != nil {
				return ctrl.Result{}, err
			}
			finalizer.Remove(obj, constants.Finalizer)
			return ctrl.Result{}, r.Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	key := types.NamespacedName{
		Name:      obj.Spec.CredentialsRef.Name,
		Namespace: obj.Spec.CredentialsRef.Namespace,
	}

	creds := &corev1.Secret{}
	if err := r.Get(ctx, key, creds); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ClusterService.Ensure(ctx, log, obj, creds); err != nil {
		return ctrl.Result{}, err
	}

	var failure error
	for i := range obj.Spec.AgentPools {
		pool := &v1alpha1.AgentPool{
			Spec: azurev1alpha1.AgentPoolSpec{
				SubscriptionID:    obj.Spec.SubscriptionID,
				ResourceGroup:     obj.Spec.ResourceGroup,
				Cluster:           obj.Spec.Name,
				AgentPoolTemplate: obj.Spec.AgentPools[i],
			},
		}
		log := log.WithValues("agentpool", pool.Spec.Name)
		if err := r.PoolService.Ensure(ctx, log, pool); err != nil {
			failure = err
			log.Error(err, fmt.Sprintf("failed to updated agent pool: %s", pool.Spec.Name))
		}
	}

	if failure != nil {
		return ctrl.Result{}, failure
	}

	var requeue bool
	// We may want to store the Kubeconfig from the ManagedCluster in a Kubernetes secret.
	// Alternatively, we may need the Kubeconfig to apply some Kustomization workloads against the cluster.
	if obj.Spec.KubeconfigRef != nil || obj.Spec.Kustomizations != nil {
		kubeconfigBytes, err := r.ClusterService.GetCredentials(context.Background(), obj.Spec.SubscriptionID, obj.Spec.ResourceGroup, obj.Spec.Name)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Construct and apply Kubeconfig as corev1.Secret
		if obj.Spec.KubeconfigRef != nil {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      obj.Spec.KubeconfigRef.Name,
					Namespace: obj.Namespace,
				},
			}
			_, err = controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
				secret.Data = map[string][]byte{
					obj.Spec.KubeconfigRef.Key: kubeconfigBytes,
				}
				return nil
			})
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// BEGIN Kustomization
		// Build and apply kustomized objects
		if obj.Spec.Kustomizations != nil {
			// Construct remote client
			// TODO(ace): simplify. If we can remove dep on clientcmd, we can run manager.exe on windows again.
			clientconfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
			if err != nil {
				return ctrl.Result{}, err
			}
			restClient, err := clientconfig.ClientConfig()
			if err != nil {
				return ctrl.Result{}, err
			}
			kubeclient, err := client.New(restClient, client.Options{
				Scheme: r.Scheme,
			})
			if err != nil {
				return ctrl.Result{}, err
			}

			// Initialize for kustomization
			// TODO(ace): does this persist anything on disk, can we use in memory instead
			// ace: could not find anything persisted to disk (uses /tmp/kustomize**** and files are deleted)
			// ace: in memory version of filesys did not work on initial attempt
			fs := filesys.MakeFsOnDisk()
			koptions := krusty.MakeDefaultOptions()
			// koptions.DoLegacyResourceSort = false
			kustomizer := krusty.MakeKustomizer(fs, koptions)

			// Potentially we may have many kustomizations to apply.
			for _, path := range obj.Spec.Kustomizations {
				// Kustomize build
				resmap, err := kustomizer.Run(path)
				if err != nil {
					return ctrl.Result{}, err
				}

				// We need to extract runtime.Objects from the Kustomize out to apply them with our kubeclient.
				// The easiest way to do so is to convert everything to yaml, and decode via separators (---)
				// This gives us an array of runtime.Objects which we can directly CreateOrUpdate on the remote cluster.
				data, err := resmap.AsYaml()
				if err != nil {
					return ctrl.Result{}, err
				}
				buf := bytes.NewBuffer(data)
				d := decoder.NewYAMLDecoder(ioutil.NopCloser(buf), r.Scheme)
				objects := []runtime.Object{}
				for {
					obj, _, err := d.Decode(nil, nil)
					if err == io.EOF {
						break
						// Note that we are ignoring runtime.IsNotRegisteredError, because we want to use unstructured
						// TODO(ace): test this...may need reworking for the intended purpose
						// n.b.: github.com/kubernetes/cli-runtime can do the unstructured with its resource builder...check that out
					} else if err != nil {
						if runtime.IsNotRegisteredError(err) {
							if obj != nil {
								log.Error(err, "failed to recognize object")
								litter.Dump(obj)
								requeue = true
							}
							continue
						}
						// } else if err != nil && !runtime.IsNotRegisteredError(err) {
						return ctrl.Result{}, err
					}
					objects = append(objects, obj)
				}

				// Apply objects output from kustomization, one by one
				for _, obj := range objects {
					if _, err := controllerutil.CreateOrUpdate(ctx, kubeclient, obj, func() error { return nil }); err != nil {
						return ctrl.Result{}, err
					}
				}
			}
		}
		// END Kustomization
	}

	return ctrl.Result{Requeue: requeue}, nil
}

func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&azurev1alpha1.ManagedCluster{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(r)
}
