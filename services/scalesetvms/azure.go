package scalesetvms

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-03-01/compute"
	"github.com/Azure/go-autorest/autorest"

	"github.com/alexeldeib/azsvc/constants"
)

type client struct {
	compute.VirtualMachineScaleSetVMsClient
}

func newClient(authorizer autorest.Authorizer, subscriptionID string) (*client, error) {
	var c = compute.NewVirtualMachineScaleSetVMsClient(subscriptionID)
	c.Authorizer = authorizer
	if err := c.AddToUserAgent(constants.UserAgent); err != nil {
		return nil, err
	}
	return &client{c}, nil
}

func (c *client) delete(ctx context.Context, group, vmss, instance string) error {
	future, err := c.VirtualMachineScaleSetVMsClient.Delete(ctx, group, vmss, instance)
	if err != nil {
		return err
	}
	return future.WaitForCompletionRef(ctx, c.Client)
}
