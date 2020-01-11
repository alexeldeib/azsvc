package scalesetvms

import (
	"context"

	"github.com/Azure/go-autorest/autorest"
	"github.com/alexeldeib/azsvc/pkg/errors"
)

type Service struct {
	authorizer autorest.Authorizer
}

func NewService(authorizer autorest.Authorizer) *Service {
	return &Service{
		authorizer,
	}
}

func (s *Service) Delete(ctx context.Context, subscriptionID, group, vmss, instance string) error {
	client, err := newClient(s.authorizer, subscriptionID)
	if err != nil {
		return err
	}
	err = client.delete(ctx, group, vmss, instance)
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	return err
}
