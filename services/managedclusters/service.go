package managedclusters

import (
	"context"
	"errors"

	azerr "github.com/alexeldeib/azsvc/errors"
	"github.com/Azure/go-autorest/autorest"
)

type Service struct {
	authorizer autorest.Authorizer
}

func NewService(authorizer autorest.Authorizer) *Service {
	return &Service{
		authorizer,
	}
}

func (s *Service) Ensure(ctx context.Context, resource *Spec) error {
	client, err := newClient(s.authorizer, resource.subscriptionID)
	if err != nil {
		return err
	}
	result, err := client.createOrUpdate(ctx, resource.group, *resource.internal.Name, resource.internal)
	if err != nil {
		return err
	}
	resource.internal = result
	return nil
}

func (s *Service) Get(ctx context.Context, subscriptionID, resourceGroup, name string) (*Spec, error) {
	client, err := newClient(s.authorizer, subscriptionID)
	if err != nil {
		return nil, err
	}

	result, err := client.get(ctx, resourceGroup, name)
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

func (s *Service) Delete(ctx context.Context, subscriptionID, resourceGroup, name string) error {
	client, err := newClient(s.authorizer, subscriptionID)
	if err != nil {
		return err
	}
	err = client.delete(ctx, resourceGroup, name)
	if err != nil && azerr.IsNotFound(err) {
		return nil
	}
	return err
}
