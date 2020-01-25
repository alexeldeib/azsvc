package managedclusters

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/alexeldeib/azsvc/api/v1alpha1"
	"github.com/alexeldeib/azsvc/pkg/autoutil"
)

const (
	backoffSteps    = 30
	backoffFactor   = 1.25
	backoffInterval = 5 * time.Second
	backoffJitter   = 1
	backoffLimit    = 2700 * time.Second
)

func backoff() wait.Backoff {
	return wait.Backoff{
		Cap:      backoffLimit,
		Steps:    backoffSteps,
		Factor:   backoffFactor,
		Duration: backoffInterval,
		Jitter:   backoffJitter,
	}
}

type Service struct {
	authorizer autorest.Authorizer
	newClient  func(autorest.Authorizer, string) (*client, error)
}

func NewService(authorizer autorest.Authorizer) *Service {
	return &Service{
		authorizer,
		newClient,
	}
}

func (s *Service) Ensure(ctx context.Context, log logr.Logger, obj *v1alpha1.ManagedCluster, creds *corev1.Secret) error {
	client, err := s.newClient(s.authorizer, obj.Spec.SubscriptionID)
	if err != nil {
		return err
	}

	log.V(1).Info("fetching azure cluster")
	spec, err := s.Get(ctx, obj.Spec.SubscriptionID, obj.Spec.ResourceGroup, obj.Spec.Name)
	if err != nil {
		return err
	}

	spec.Set(
		Name(obj.Spec.Name),
		Location(obj.Spec.Location),
		SubscriptionID(obj.Spec.SubscriptionID),
		ResourceGroup(obj.Spec.ResourceGroup),
		KubernetesVersion(obj.Spec.Version),
		DNSPrefix(obj.Spec.Name),
		ServicePrincipal(string(creds.Data[auth.ClientID]), string(creds.Data[auth.ClientSecret])),
		SSHPublicKey(obj.Spec.SSHPublicKey),
	)

	log.V(1).Info("checking if cluster exists")
	if !spec.Exists() {
		for _, pool := range obj.Spec.AgentPools {
			spec.Set(
				AgentPool(pool.Name, pool.SKU, pool.Replicas, pool.OSDiskSizeGB),
			)
		}
	}

	if spec.internal.ProvisioningState != nil && *spec.internal.ProvisioningState != "Succeeded" && *spec.internal.ProvisioningState != "Failed" {
		return fmt.Errorf("resource still in provisioning state: %s", *spec.internal.ProvisioningState)
	}

	log.V(1).Info("fetching azure cluster")
	diff := spec.Diff()
	if diff == "" && spec.internal.ProvisioningState != nil && *spec.internal.ProvisioningState != "Failed" {
		log.V(2).Info("no update required, found and desired objects equal")
		return nil
	}
	fmt.Printf("update required (+want -have):\n%s", diff)

	log.V(1).Info("beginning long create/update operation")
	_, err = client.createOrUpdate(ctx, log, obj.Spec.ResourceGroup, obj.Spec.Name, *spec.internal)
	return err
}

func (s *Service) Delete(ctx context.Context, log logr.Logger, obj *v1alpha1.ManagedCluster) error {
	client, err := s.newClient(s.authorizer, obj.Spec.SubscriptionID)
	if err != nil {
		return err
	}

	log.V(1).Info("beginning long delete operation")
	err = client.delete(ctx, log, obj.Spec.ResourceGroup, obj.Spec.Name)
	if err != nil {
		if autoutil.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (s *Service) Get(ctx context.Context, subscriptionID, resourceGroup, name string) (*Spec, error) {
	client, err := s.newClient(s.authorizer, subscriptionID)
	if err != nil {
		return nil, err
	}

	result, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		if autoutil.IsNotFound(err) {
			return defaultSpec(name), nil
		}
		return nil, err
	}

	return &Spec{
		internal: &result,
		old:      &result,
	}, nil
}

func (s *Service) GetCredentials(ctx context.Context, subscriptionID, resourceGroup, name string) ([]byte, error) {
	client, err := s.newClient(s.authorizer, subscriptionID)
	if err != nil {
		return nil, err
	}
	return client.getKubeconfig(ctx, resourceGroup, name)
}
