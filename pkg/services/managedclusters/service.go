package managedclusters

import (
	"context"
	"errors"
	"time"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/alexeldeib/azsvc/api/v1alpha1"
	azerr "github.com/alexeldeib/azsvc/pkg/errors"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
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
	log        logr.Logger
}

func NewService(authorizer autorest.Authorizer, logger logr.Logger) *Service {
	return &Service{
		authorizer,
		logger,
	}
}

func (s *Service) Ensure(ctx context.Context, obj *v1alpha1.ManagedCluster) error {
	client, err := newClient(s.authorizer, obj.Spec.SubscriptionID)
	if err != nil {
		return err
	}

	spec, err := s.Get(ctx, obj.Spec.SubscriptionID, obj.Spec.ResourceGroup, obj.Spec.Name)
	if err != nil {
		return err
	}

	settings, err := auth.GetSettingsFromFile()
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
		ServicePrincipal(settings.Values[auth.ClientID], settings.Values[auth.ClientSecret]),
		SSHPublicKey(obj.Spec.SSHPublicKey),
	)

	if !spec.Exists() {
		for _, pool := range obj.Spec.AgentPools {
			spec.Set(
				AgentPool(pool.Name, pool.SKU, pool.Replicas, pool.OSDiskSizeGB),
			)
		}
	}

	s.log.Info("beginning long create/update operation")
	future, err := client.CreateOrUpdate(ctx, obj.Spec.ResourceGroup, obj.Spec.Name, spec.internal)
	if err != nil {
		return err
	}

	return wait.ExponentialBackoff(backoff(), func() (done bool, err error) {
		s.log.Info("reconciling with backoff")
		done, err = future.DoneWithContext(ctx, client)
		if err != nil {
			s.log.Error(err, "failed reconcile attempt")
		}
		return done && err == nil, nil
	})
}

func (s *Service) Delete(ctx context.Context, obj *v1alpha1.ManagedCluster) error {
	client, err := newClient(s.authorizer, obj.Spec.SubscriptionID)
	if err != nil {
		return err
	}

	s.log.Info("beginning long delete operation")
	future, err := client.Delete(ctx, obj.Spec.ResourceGroup, obj.Spec.Name)
	if err != nil {
		if azerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	return wait.ExponentialBackoff(backoff(), func() (done bool, err error) {
		s.log.Info("deleting with backoff")
		done, err = future.DoneWithContext(ctx, client)
		if err != nil {
			s.log.Error(err, "failed deletion attempt")
		}
		return done && err == nil, nil
	})
}

func (s *Service) Get(ctx context.Context, subscriptionID, resourceGroup, name string) (*Spec, error) {
	client, err := newClient(s.authorizer, subscriptionID)
	if err != nil {
		return nil, err
	}

	result, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		if azerr.IsNotFound(err) {
			return defaultSpec(), nil
		}
		return nil, err
	}

	return &Spec{
		internal: result,
	}, nil
}

func (s *Service) GetCredentials(ctx context.Context, subscriptionID, resourceGroup, name string) ([]byte, error) {
	client, err := newClient(s.authorizer, subscriptionID)
	if err != nil {
		return nil, err
	}

	credentialList, err := client.ListClusterAdminCredentials(ctx, resourceGroup, name)
	if err != nil {
		return nil, err
	}

	if credentialList.Kubeconfigs == nil || len(*credentialList.Kubeconfigs) < 1 {
		return nil, errors.New("no kubeconfigs available for the aks cluster")
	}

	return *(*credentialList.Kubeconfigs)[0].Value, nil
}
