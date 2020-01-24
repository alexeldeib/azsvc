package managedclusters

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-11-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/alexeldeib/azsvc/pkg/constants"
)

type client struct {
	containerservice.ManagedClustersClient
}

func newClient(authorizer autorest.Authorizer, subscriptionID string) (*client, error) {
	var c = containerservice.NewManagedClustersClient(subscriptionID)
	c.Authorizer = authorizer
	c.PollingDuration = 45 * time.Minute
	if err := c.AddToUserAgent(constants.UserAgent); err != nil {
		return nil, err
	}
	return &client{c}, nil
}

func (c *client) createOrUpdate(ctx context.Context, log logr.Logger, group, name string, properties containerservice.ManagedCluster) (containerservice.ManagedCluster, error) {
	future, err := c.ManagedClustersClient.CreateOrUpdate(ctx, group, name, properties)
	if err != nil {
		return containerservice.ManagedCluster{}, err
	}
	if err := poll(ctx, log, c.Client, future.Future); err != nil {
		return containerservice.ManagedCluster{}, err
	}
	return future.Result(c.ManagedClustersClient)
}

func (c *client) get(ctx context.Context, group, name string) (containerservice.ManagedCluster, error) {
	return c.ManagedClustersClient.Get(ctx, group, name)
}

func (c *client) delete(ctx context.Context, log logr.Logger, group, name string) error {
	future, err := c.ManagedClustersClient.Delete(ctx, group, name)
	if err != nil {
		return err
	}
	return poll(ctx, log, c.Client, future.Future)
}

func (c *client) getKubeconfig(ctx context.Context, group, name string) ([]byte, error) {
	credentialList, err := c.ManagedClustersClient.ListClusterAdminCredentials(ctx, group, name)
	if err != nil {
		return nil, err
	}
	if credentialList.Kubeconfigs == nil || len(*credentialList.Kubeconfigs) < 1 {
		return nil, errors.New("no kubeconfigs available for the aks cluster")
	}
	return *(*credentialList.Kubeconfigs)[0].Value, nil
}

func poll(ctx context.Context, log logr.Logger, client autorest.Client, future azure.Future) error {
	return wait.ExponentialBackoff(backoff(), func() (done bool, err error) {
		log.Info("reconciling with backoff")
		done, err = future.DoneWithContext(ctx, client)
		if err != nil {
			log.Info(fmt.Sprintf("failed reconcile attempt, %s", err.Error()))
		}
		return done, err
	})
}

func addDebug(client *autorest.Client) {
	client.RequestInspector = logRequest()
	client.ResponseInspector = logResponse()
}

// logRequest logs full autorest requests for any Azure client.
func logRequest() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r, err := p.Prepare(r)
			if err != nil {
				fmt.Println(err)
			}
			dump, _ := httputil.DumpRequestOut(r, true)
			fmt.Println(string(dump))
			return r, err
		})
	}
}

// logResponse logs full autorest responses for any Azure client.
func logResponse() autorest.RespondDecorator {
	return func(p autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(r *http.Response) error {
			err := p.Respond(r)
			if err != nil {
				fmt.Println(err)
			}
			dump, _ := httputil.DumpResponse(r, true)
			fmt.Println(string(dump))
			return err
		})
	}
}
