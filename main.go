package main

import (
	"flag"
	"os"

	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	azurev1alpha1 "github.com/alexeldeib/azsvc/api/v1alpha1"
	"github.com/alexeldeib/azsvc/controllers"
	realzap "go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/alexeldeib/azsvc/pkg/services/agentpools"
	"github.com/alexeldeib/azsvc/pkg/services/managedclusters"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)
	_ = apiextensionsv1beta1.AddToScheme(scheme)

	_ = azurev1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var debug bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&debug, "debug", false, "more verbose logs will print when debug is true.")
	flag.Parse()

	ctrl.SetLogger(
		zap.New(
			zap.RawZapOpts(realzap.AddCaller()),
			func(o *zap.Options) {
				o.Development = debug
			},
		),
	)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	settings, err := auth.GetSettingsFromFile()
	if err != nil {
		setupLog.Error(err, "unable to fetch azure settings")
		os.Exit(1)
	}

	authorizer, err := settings.ClientCredentialsAuthorizer(azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		setupLog.Error(err, "unable to fetch azure authorizer")
		os.Exit(1)
	}

	clusterService := managedclusters.NewService(authorizer)
	poolService := agentpools.NewService(authorizer)

	if err = (&controllers.ManagedClusterReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("ManagedCluster"),
		Scheme:         mgr.GetScheme(),
		ClusterService: clusterService,
		PoolService:    poolService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagedCluster")
		os.Exit(1)
	}
	if err = (&controllers.AgentPoolReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("AgentPool"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AgentPool")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
