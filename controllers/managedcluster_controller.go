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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/tools/clientcmd"
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

type Object interface {
	metav1.Object
	runtime.Object
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=azure.alexeldeib.xyz,resources=managedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=azure.alexeldeib.xyz,resources=managedclusters/status,verbs=get;update;patch

func (r *ManagedClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcluster", req.NamespacedName)

	managedCluster := &azurev1alpha1.ManagedCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, managedCluster); err != nil {
		log.Info("failed to fetch managed cluster", "error", err.Error())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.V(2).Info("checking deletion timestamp")
	if managedCluster.GetDeletionTimestamp().IsZero() {
		log.V(2).Info("will try to add finalizer")
		if !finalizer.Has(managedCluster, constants.Finalizer) {
			finalizer.Add(managedCluster, constants.Finalizer)
			log.V(1).Info("added finalizer")
			return ctrl.Result{}, r.Update(ctx, managedCluster)
		}
	} else {
		log.V(2).Info("checking for finalizer")
		if finalizer.Has(managedCluster, constants.Finalizer) {
			log.V(1).Info("finalizer present, invoking deletion")
			if err := r.ClusterService.Delete(ctx, log, managedCluster); err != nil {
				return ctrl.Result{}, err
			}
			finalizer.Remove(managedCluster, constants.Finalizer)
			return ctrl.Result{}, r.Update(ctx, managedCluster)
		}
		return ctrl.Result{}, nil
	}

	key := types.NamespacedName{
		Name:      managedCluster.Spec.CredentialsRef.Name,
		Namespace: managedCluster.Spec.CredentialsRef.Namespace,
	}

	creds := &corev1.Secret{}
	if err := r.Get(ctx, key, creds); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ClusterService.Ensure(ctx, log, managedCluster, creds); err != nil {
		return ctrl.Result{}, err
	}

	var failure error
	for i := range managedCluster.Spec.AgentPools {
		pool := &v1alpha1.AgentPool{
			Spec: azurev1alpha1.AgentPoolSpec{
				SubscriptionID:    managedCluster.Spec.SubscriptionID,
				ResourceGroup:     managedCluster.Spec.ResourceGroup,
				Cluster:           managedCluster.Spec.Name,
				AgentPoolTemplate: managedCluster.Spec.AgentPools[i],
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
	if managedCluster.Spec.KubeconfigRef != nil || managedCluster.Spec.Kustomizations != nil || managedCluster.Spec.Manifests != nil || managedCluster.Status.Applied != nil {
		kubeconfigBytes, err := r.ClusterService.GetCredentials(context.Background(), managedCluster.Spec.SubscriptionID, managedCluster.Spec.ResourceGroup, managedCluster.Spec.Name)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Construct and apply Kubeconfig as corev1.Secret
		if managedCluster.Spec.KubeconfigRef != nil {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedCluster.Spec.KubeconfigRef.Name,
					Namespace: managedCluster.Namespace,
				},
			}
			_, err = controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
				secret.Data = map[string][]byte{
					managedCluster.Spec.KubeconfigRef.Key: kubeconfigBytes,
				}
				return nil
			})
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		var runObjs []runtime.Object

		// Build kubeconfig for remote workload cluster
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

		// Construct map of previously applied objects
		diff := map[corev1.ObjectReference]bool{}
		for _, ref := range managedCluster.Status.Applied {
			diff[ref] = true
		}

		getter, err := remote.NewRESTClientGetter(kubeconfigBytes)
		if err != nil {
			return ctrl.Result{}, err
		}
		factory := cmdutil.NewFactory(getter)

		for i := range managedCluster.Spec.Kustomizations {
			url := managedCluster.Spec.Kustomizations[i]
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

			v := factory.NewBuilder().
				Unstructured().
				Schema(opts.Validator).
				ContinueOnError().
				NamespaceParam(opts.Namespace).DefaultNamespace().
				FilenameParam(opts.EnforceNamespace, &opts.DeleteOptions.FilenameOptions).
				LabelSelectorParam(opts.Selector).
				Flatten().
				Do()

			err = v.Visit(func(info *resource.Info, err error) error {
				if err != nil {
					return err
				}
				runObjs = append(runObjs, info.Object)
				return nil
			})

			if err := opts.Run(); err != nil {
				log.Error(err, "failed to apply")
				return ctrl.Result{}, err
			}

			log.V(2).Info("output", "stdio", stdio.String(), "errio", errio.String())
		}

		if managedCluster.Spec.Manifests != nil {
			stdio := bytes.NewBuffer(nil)
			errio := bytes.NewBuffer(nil)
			streams := genericclioptions.IOStreams{In: stdio, Out: stdio, ErrOut: errio}
			cmd := apply.NewCmdApply("kubectl", factory, streams)
			opts := apply.NewApplyOptions(streams)
			opts.DeleteFlags.FileNameFlags.Filenames = &managedCluster.Spec.Manifests

			if err := opts.Complete(factory, cmd); err != nil {
				log.Error(err, "failed to complete apply options")
				return ctrl.Result{}, err
			}

			v := factory.NewBuilder().
				Unstructured().
				Schema(opts.Validator).
				ContinueOnError().
				NamespaceParam(opts.Namespace).DefaultNamespace().
				FilenameParam(opts.EnforceNamespace, &opts.DeleteOptions.FilenameOptions).
				LabelSelectorParam(opts.Selector).
				Flatten().
				Do()

			err = v.Visit(func(info *resource.Info, err error) error {
				if err != nil {
					return err
				}
				runObjs = append(runObjs, info.Object)
				return nil
			})

			if err := opts.Run(); err != nil {
				log.Error(err, "failed to apply")
				return ctrl.Result{}, err
			}

			log.V(2).Info("output", "stdio", stdio.String(), "errio", errio.String())
		}

		for i := range managedCluster.Spec.ObjectRefs {
			ref := managedCluster.Spec.ObjectRefs[i]
			key := types.NamespacedName{
				Namespace: ref.Namespace,
				Name:      ref.Name,
			}

			obj := new(unstructured.Unstructured)
			obj.SetAPIVersion(ref.APIVersion)
			obj.SetKind(ref.Kind)
			if err := r.Get(ctx, key, obj); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to get object ref to slingshot")
			}

			runObjs = append(runObjs, obj)

			old := obj.DeepCopy()
			zero := metav1.Time{
				Time: time.Time{},
			}
			old.SetCreationTimestamp(zero)
			old.SetUID("")
			old.SetResourceVersion("")

			if _, err := controllerutil.CreateOrUpdate(ctx, kubeclient, old, func() error {
				timestamp := old.GetCreationTimestamp()
				rv := old.GetResourceVersion()
				uid := old.GetUID()
				old.SetUnstructuredContent(obj.UnstructuredContent())
				old.SetCreationTimestamp(timestamp)
				old.SetUID(uid)
				old.SetResourceVersion(rv)

				return nil
			}); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to apply local object to remote cluster")
			}
			runObjs = append(runObjs, obj)
		}

		// set difference old and new applied objects
		managedCluster.Status.Applied = make([]corev1.ObjectReference, len(runObjs))
		for i := range runObjs {
			obj := runObjs[i].(Object)
			log.V(2).Info("removing gvk from diff", "gvk", obj.GetObjectKind().GroupVersionKind().String(), "name", obj.GetName(), "namespace", obj.GetNamespace())
			apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
			ref := corev1.ObjectReference{
				Name:       obj.GetName(),
				Namespace:  obj.GetNamespace(),
				Kind:       kind,
				APIVersion: apiVersion,
			}
			delete(diff, ref)
			managedCluster.Status.Applied = append(managedCluster.Status.Applied)
		}

		// Delete old objects
		for ref := range diff {
			obj := new(unstructured.Unstructured)
			obj.SetAPIVersion(ref.APIVersion)
			obj.SetKind(ref.Kind)
			obj.SetName(ref.Name)
			obj.SetNamespace(ref.Namespace)
			if err := kubeclient.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
				if meta.IsNoMatchError(err) {
					log.V(2).Info("gvk not found, crd likely not installed. will continue")
					log.V(2).Info(err.Error())
					continue
				}
				log.Error(err, "failed to delete previously applied item in diff")
				return ctrl.Result{}, err
			}
			log.Info("deleted object", "name", ref.Name, "namespace", ref.Namespace, "kind", ref.Kind, "apiVersion", ref.APIVersion)
		}

		// Apply status
		if err := r.Status().Update(ctx, managedCluster); err != nil {
			log.Error(err, "failed to update managed cluster status")
			return ctrl.Result{}, err
		}
	}

	log.V(1).Info("successfully reconciled")
	return ctrl.Result{RequeueAfter: time.Second * 60}, nil
}

func decode(r *apiyaml.YAMLReader, log logr.Logger) ([]*unstructured.Unstructured, error) {
	var out []*unstructured.Unstructured
	for {
		raw, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		var yamldata map[string]interface{}
		if fail := yaml.Unmarshal(raw, &yamldata); fail != nil {
			log.Error(fail, "failed to unmarshal object as unstructured")
			return nil, fail
		}
		unstruct := &unstructured.Unstructured{
			Object: yamldata,
		}
		out = append(out, unstruct)
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
