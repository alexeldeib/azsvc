package agentpools

import (
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-11-01/containerservice"
)

var defaultUser string = "azureuser"
var emptyString string = ""

type specOption func(*Spec) *Spec

type Spec struct {
	subscriptionID string
	group          string
	cluster        string
	internal       containerservice.AgentPool
}

func defaultSpec() *Spec {
	result := &Spec{
		internal: containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				Type: containerservice.VirtualMachineScaleSets,
			},
		},
	}
	return result
}

func (s *Spec) Set(options ...specOption) {
	for _, option := range options {
		s = option(s)
	}
}

func (s *Spec) Exists() bool {
	return s.internal.ID != nil
}

func Name(name string) specOption {
	return func(o *Spec) *Spec {
		o.internal.Name = &name
		return o
	}
}

func Cluster(cluster string) specOption {
	return func(o *Spec) *Spec {
		o.cluster = cluster
		return o
	}
}

func SubscriptionID(sub string) specOption {
	return func(o *Spec) *Spec {
		o.subscriptionID = sub
		return o
	}
}

func ResourceGroup(group string) specOption {
	return func(o *Spec) *Spec {
		o.group = group
		return o
	}
}

func KubernetesVersion(version string) specOption {
	return func(o *Spec) *Spec {
		o.internal.OrchestratorVersion = &version
		return o
	}
}

func Count(count int32) specOption {
	return func(o *Spec) *Spec {
		o.internal.Count = &count
		return o
	}
}

func SKU(sku string) specOption {
	return func(o *Spec) *Spec {
		o.internal.VMSize = containerservice.VMSizeTypes(sku)
		return o
	}
}

func OSDiskSizeGB(size *int32) specOption {
	return func(o *Spec) *Spec {
		o.internal.OsDiskSizeGB = size
		return o
	}
}

func (s *Spec) Count() int32 {
	if s.internal.Count == nil {
		return 0
	}
	return *s.internal.Count
}
