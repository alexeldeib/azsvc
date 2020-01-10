package managedclusters_test

import (
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	settings   auth.FileSettings
	log        logr.Logger
	authorizer autorest.Authorizer
)

func TestManagedclusters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Managedclusters Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, false))
	log = logf.Log.WithName("managedcluster_suite")

	var err error
	settings, err = auth.GetSettingsFromFile()
	Expect(err).NotTo(HaveOccurred())

	log.WithValues(
		"subscription", settings.Values[auth.SubscriptionID],
		"app", settings.Values[auth.ClientID],
	).Info("using client configuration")

	authorizer, err = settings.ClientCredentialsAuthorizer(azure.PublicCloud.ResourceManagerEndpoint)
	Expect(err).NotTo(HaveOccurred())

})

// var _ = AfterSuite(func() {
//
// })
