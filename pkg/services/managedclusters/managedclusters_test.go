package managedclusters_test

import (
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
)

var (
	// regex used by azure is `^[a-zA-Z0-9]$|^[a-zA-Z0-9][-_a-zA-Z0-9]{0,61}[a-zA-Z0-9]$` (i think?)
	// normalize to a standard length of something similar
	validResourceName string = `^ace-capi-[a-zA-Z0-9]{5}$`
	location                 = "westus2"
	resourceGroup            = "ace-break"
	version                  = "1.16.4"
	bitSize           int    = 4096
)

var _ = Describe("Managedclusters", func() {
	It("can create a cluster with two agent pools using managed cluster api", func() {
		// svc = NewManagedClusterService(authorizer, &http.Client{})

		// g, err := reggen.NewGenerator(validResourceName)
		// Expect(err).NotTo(HaveOccurred())
		// _ = g

		// // Cluster name
		// var name = g.Generate(2)
		// // var name = "ace-capi-zrha8"

		// // Dummy SSH key
		// privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(privateKey.Validate()).To(Succeed())

		// publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
		// Expect(err).NotTo(HaveOccurred())

		// // Throw away, extract public
		// privateKey = nil
		// publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)
		// _ = publicKeyBytes

		// By("fetching cluster (should 404 and return default spec)")
		// spec, err := svc.Get(context.Background(), settings.Values[auth.SubscriptionID], resourceGroup, name)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(spec.Exists()).To(BeFalse())

		// By("Creating cluster with two pools")
		// spec.Set(
		// 	Name(name),
		// 	Location(location),
		// 	SubscriptionID(settings.Values[auth.SubscriptionID]),
		// 	ResourceGroup(resourceGroup),
		// 	KubernetesVersion(version),
		// 	DNSPrefix(name),
		// 	WithServicePrincipal(settings.Values[auth.ClientID], settings.Values[auth.ClientSecret]),
		// 	WithAgentPool("main", "Standard_D4_v3", 1, 500),
		// 	WithAgentPool("big", "Standard_D8_v3", 1, 500),
		// 	SSHPublicKey(string(publicKeyBytes)),
		// )

		// err = svc.CreateOrUpdate(context.Background(), settings.Values[auth.SubscriptionID], resourceGroup, name, spec)
		// if err != nil {
		// 	Fail(err.Error())
		// }

		// By("Adding pool to spec and applying")
		// spec.Set(
		// 	WithAgentPool("big", "Standard_D4_v3", 1, 500),
		// )

		// err = svc.CreateOrUpdate(context.Background(), settings.Values[auth.SubscriptionID], resourceGroup, name, spec)
		// Expect(err).To(HaveOccurred())

		// done, err := svc.Delete(context.Background(), settings.Values[auth.SubscriptionID], resourceGroup, name)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(done).To(BeTrue())
	})
})
