package agentpools

import (
	"context"

	"github.com/Azure/go-autorest/autorest"

	"github.com/alexeldeib/azsvc/errors"
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
	return client.createOrUpdate(ctx, resource.group, resource.cluster, *resource.internal.Name, resource.internal)
}

func (s *Service) Get(ctx context.Context, subscriptionID, resourceGroup, cluster, name string) (*Spec, error) {
	client, err := newClient(s.authorizer, subscriptionID)
	if err != nil {
		return nil, err
	}

	result, err := client.get(ctx, resourceGroup, cluster, name)
	if err != nil {
		if errors.IsNotFound(err) {
			return defaultSpec(), nil
		}
		return nil, err
	}

	return &Spec{
		internal: result,
	}, nil
}

func (s *Service) Delete(ctx context.Context, subscriptionID, resourceGroup, cluster, name string) error {
	client, err := newClient(s.authorizer, subscriptionID)
	if err != nil {
		return err
	}
	err = client.delete(ctx, resourceGroup, cluster, name)
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	return err
}
