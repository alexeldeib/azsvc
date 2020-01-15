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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	"github.com/alexeldeib/azsvc/api/v1alpha1"
	azurev1alpha1 "github.com/alexeldeib/azsvc/api/v1alpha1"
	"github.com/alexeldeib/azsvc/pkg/constants"
	"github.com/alexeldeib/azsvc/pkg/decoder"
	"github.com/alexeldeib/azsvc/pkg/finalizer"
	"github.com/alexeldeib/azsvc/pkg/remote"
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
			log.V(1).Info("added finalizer")
			return ctrl.Result{}, r.Update(ctx, obj)
		}
	} else {
		log.V(2).Info("checking for finalizer")
		if finalizer.Has(obj, constants.Finalizer) {
			log.V(1).Info("finalizer present, invoking deletion")
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

	// We may want to store the Kubeconfig from the ManagedCluster in a Kubernetes secret.
	// Alternatively, we may need the Kubeconfig to apply some Kustomization workloads against the cluster.
	if obj.Spec.KubeconfigRef != nil || obj.Spec.Kustomizations != nil || obj.Spec.Manifests != nil {
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

		if obj.Spec.Kustomizations != nil || obj.Spec.Manifests != nil {
			getter, err := remote.NewRESTClientGetter(kubeconfigBytes)
			if err != nil {
				return ctrl.Result{}, err
			}

			factory := cmdutil.NewFactory(getter)

			for i := range obj.Spec.Kustomizations {
				url := obj.Spec.Kustomizations[i]
				stdio := bytes.NewBuffer(nil)
				errio := bytes.NewBuffer(nil)
				streams := genericclioptions.IOStreams{In: stdio, Out: stdio, ErrOut: errio}
				cmd := apply.NewCmdApply("kubectl", factory, streams)
				opts := apply.NewApplyOptions(streams)
				opts.DeleteFlags.FileNameFlags.Kustomize = &url

				if err := opts.Complete(factory, cmd); err != nil {
					log.Error(err, "failed to complete apply options")
					return ctrl.Result{}, err
				}

				if err := opts.Run(); err != nil {
					log.Error(err, "failed to apply")
					return ctrl.Result{}, err
				}

				log.V(2).Info("output", "stdio", stdio.String(), "errio", errio.String())
			}

			if obj.Spec.Manifests != nil {
				stdio := bytes.NewBuffer(nil)
				errio := bytes.NewBuffer(nil)
				streams := genericclioptions.IOStreams{In: stdio, Out: stdio, ErrOut: errio}
				cmd := apply.NewCmdApply("kubectl", factory, streams)
				opts := apply.NewApplyOptions(streams)
				opts.DeleteFlags.FileNameFlags.Filenames = &obj.Spec.Manifests

				if err := opts.Complete(factory, cmd); err != nil {
					log.Error(err, "failed to complete apply options")
					return ctrl.Result{}, err
				}

				if err := opts.Run(); err != nil {
					log.Error(err, "failed to apply")
					return ctrl.Result{}, err
				}

				log.V(2).Info("output", "stdio", stdio.String(), "errio", errio.String())
			}
		}
	}
	log.V(1).Info("successfully reconciled")
	return ctrl.Result{RequeueAfter: time.Second * 60}, nil
}

func decode(d *decoder.YamlDecoder, log logr.Logger) ([]runtime.Object, error) {
	var out []runtime.Object
	for {
		obj, raw, err := d.Decode(nil, nil)
		if err == io.EOF {
			break
		} else if err != nil {
			if runtime.IsNotRegisteredError(err) {
				log.V(1).Info("failed to recognize object")
				log.V(1).Info(err.Error())
				var yamldata map[string]interface{}
				if fail := yaml.Unmarshal(raw, &yamldata); fail != nil {
					log.Error(fail, "failed to unmarshal object as unstructured")
					return nil, fail
				}
				unstruct := &unstructured.Unstructured{
					Object: yamldata,
				}
				out = append(out, unstruct)
				continue
			}
			return nil, err
		}
		out = append(out, obj)
	}
	return out, nil
}

func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&azurev1alpha1.ManagedCluster{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(r)
}
