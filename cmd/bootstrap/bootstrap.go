package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/go-logr/zapr"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/alexeldeib/azsvc/api/v1alpha1"
	"github.com/alexeldeib/azsvc/pkg/services/agentpools"
	"github.com/alexeldeib/azsvc/pkg/services/managedclusters"
)

func main() {
	_ = v1alpha1.AddToScheme(clientgoscheme.Scheme)

	// add flags for server connection (--kubeconfig, --server, etc)
	// apparently required even for local use, despite zero server connection?
	config := genericclioptions.NewConfigFlags(true)
	config.AddFlags(pflag.CommandLine)

	// add flags for resource building (-f, --filename, etc)
	builderFlags := genericclioptions.NewResourceBuilderFlags().
		WithFile(true).
		WithScheme(clientgoscheme.Scheme).
		WithLocal(true)
	builderFlags.AddFlags(pflag.CommandLine)

	zapLog, err := zap.NewDevelopment(zap.AddCaller())
	if err != nil {
		fmt.Printf("failed to initialize logger: %v", err)
		os.Exit(1)
	}
	log := zapr.NewLogger(zapLog)

	var (
		app, key, tenant string
	)

	pflag.StringVar(&app, "app", "", "app id to authenticate with")
	pflag.StringVar(&key, "key", "", "secret key to authenticate with")
	pflag.StringVar(&tenant, "tenant", "", "tenant id to authenticate with")

	// Parse flags
	if err := pflag.CommandLine.Parse(os.Args); err != nil {
		return
	}

	// log.Info("authenticating with values", "app", app, "tenant", tenant, "len(key)", len(key))

	authorizer, err := auth.NewClientCredentialsConfig(app, key, tenant).Authorizer()
	if err != nil {
		log.Error(err, "failed to acquire authorizer")
		os.Exit(1)
	}

	// Visit results
	visitor := builderFlags.ToBuilder(config, nil).Do()
	result, ok := visitor.(*resource.Result)
	if !ok {
		log.Error(err, "expected  result to be of rypt *resource.Result, but was not.")
		os.Exit(1)
	}

	output, err := result.Object()
	if err != nil {
		log.Error(err, "failed to build runtime object from result")
		os.Exit(1)
	}

	object, ok := output.(*v1alpha1.ManagedCluster)
	if !ok {
		log.Info("failed to find v1alpha1.ManagedCluster when expected")
		os.Exit(1)
	}

	creds := &corev1.Secret{
		Data: map[string][]byte{
			auth.ClientID:     []byte(app),
			auth.ClientSecret: []byte(key),
		},
	}

	log = log.WithValues("gvk", object.GroupVersionKind().String(), "name", object.Name)
	clusterSvc := managedclusters.NewService(authorizer, log)
	if err := clusterSvc.Ensure(context.Background(), object, creds); err != nil {
		log.Error(err, "failed to reconcile cluster")
	} else {
		log.Info("reconciled successfully")
	}

	kubeconfig, err := clusterSvc.GetCredentials(context.Background(), object.Spec.SubscriptionID, object.Spec.ResourceGroup, object.Spec.Name)
	if err != nil {
		log.Error(err, "failed to fetch kubeconfig")
	}

	if err := ioutil.WriteFile("./kubeconfig", kubeconfig, 0644); err != nil {
		log.Error(err, "failed to write kubeconfig")
		os.Exit(1)
	}

	for i := range object.Spec.AgentPools {
		val := object.Spec.AgentPools[i]
		val.SubscriptionID = object.Spec.SubscriptionID
		val.ResourceGroup = object.Spec.ResourceGroup
		val.Cluster = object.Spec.Name
		pool := &v1alpha1.AgentPool{
			Spec: val,
		}
		log := log.WithValues("agentpool", pool.Spec.Name)
		if err := agentpools.NewService(authorizer, log).Ensure(context.Background(), pool); err != nil {
			failed = true
			log.Error(err, fmt.Sprintf("failed to updated agent pool: %s", val.Name))
		}
	}

	if failed {
		os.Exit(1)
	}
}
