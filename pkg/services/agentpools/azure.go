package agentpools

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-11-01/containerservice"
	"github.com/Azure/go-autorest/autorest"

	"github.com/alexeldeib/azsvc/pkg/constants"
)

type client struct {
	containerservice.AgentPoolsClient
}

func newClient(authorizer autorest.Authorizer, subscriptionID string) (*client, error) {
	var c = containerservice.NewAgentPoolsClient(subscriptionID)
	c.Authorizer = authorizer
	if err := c.AddToUserAgent(constants.UserAgent); err != nil {
		return nil, err
	}
	return &client{c}, nil
}

func (c *client) createOrUpdate(ctx context.Context, group, cluster, name string, properties containerservice.AgentPool) error {
	future, err := c.AgentPoolsClient.CreateOrUpdate(ctx, group, cluster, name, properties)
	if err != nil {
		return err
	}
	return future.WaitForCompletionRef(ctx, c.Client)
}

func (c *client) get(ctx context.Context, group, cluster, name string) (containerservice.AgentPool, error) {
	return c.AgentPoolsClient.Get(ctx, group, cluster, name)
}

func (c *client) delete(ctx context.Context, group, cluster, name string) error {
	future, err := c.AgentPoolsClient.Delete(ctx, group, cluster, name)
	if err != nil {
		return err
	}
	return future.WaitForCompletionRef(ctx, c.Client)
}
