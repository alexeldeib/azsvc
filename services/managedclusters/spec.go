package managedclusters

import (
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-11-01/containerservice"
)

var defaultUser string = "azureuser"
var emptyString string = ""

type specOption func(*Spec) *Spec

type Spec struct {
	subscriptionID string
	group          string
	internal       containerservice.ManagedCluster
}

func defaultSpec() *Spec {
	result := &Spec{
		internal: containerservice.ManagedCluster{
			ManagedClusterProperties: &containerservice.ManagedClusterProperties{
				LinuxProfile: &containerservice.LinuxProfile{
					AdminUsername: &defaultUser,
					SSH: &containerservice.SSHConfiguration{
						PublicKeys: &[]containerservice.SSHPublicKey{},
					},
				},
				ServicePrincipalProfile: &containerservice.ManagedClusterServicePrincipalProfile{},
				AgentPoolProfiles:       &[]containerservice.ManagedClusterAgentPoolProfile{},
				NetworkProfile: &containerservice.NetworkProfileType{
					NetworkPlugin: containerservice.Azure,
				},
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

func (s *Spec) FQDN() *string {
	return s.internal.Fqdn
}

func Name(name string) specOption {
	return func(o *Spec) *Spec {
		o.internal.Name = &name
		return o
	}
}

func Location(location string) specOption {
	return func(o *Spec) *Spec {
		o.internal.Location = &location
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
		o.internal.KubernetesVersion = &version
		return o
	}
}

func DNSPrefix(prefix string) specOption {
	return func(o *Spec) *Spec {
		o.internal.DNSPrefix = &prefix
		return o
	}
}

func LoadBalancerSKU(sku string) specOption {
	return func(o *Spec) *Spec {
		o.internal.NetworkProfile.LoadBalancerSku = containerservice.LoadBalancerSku(sku)
		return o
	}
}

func NetworkPlugin(plugin string) specOption {
	return func(o *Spec) *Spec {
		o.internal.NetworkProfile.NetworkPlugin = containerservice.NetworkPlugin(plugin)
		return o
	}
}

func NetworkPolicy(policy string) specOption {
	return func(o *Spec) *Spec {
		o.internal.NetworkProfile.NetworkPolicy = containerservice.NetworkPolicy(policy)
		return o
	}
}

func PodCIDR(cidr string) specOption {
	return func(o *Spec) *Spec {
		o.internal.NetworkProfile.PodCidr = &cidr
		return o
	}
}

func ServiceCIDR(cidr string) specOption {
	return func(o *Spec) *Spec {
		o.internal.NetworkProfile.ServiceCidr = &cidr
		return o
	}
}

func DNSServiceIP(ipAddress string) specOption {
	return func(o *Spec) *Spec {
		o.internal.NetworkProfile.DNSServiceIP = &ipAddress
		return o
	}
}

func ManagedIdentity() specOption {
	return func(o *Spec) *Spec {
		o.internal.Identity.Type = containerservice.SystemAssigned
		return o
	}
}

func ServicePrincipal(app, secret string) specOption {
	return func(o *Spec) *Spec {
		o.internal.ServicePrincipalProfile.ClientID = &app
		o.internal.ServicePrincipalProfile.Secret = &secret
		return o
	}
}

func SSHPublicKey(sshKey string) specOption {
	return func(o *Spec) *Spec {
		o.internal.LinuxProfile.SSH.PublicKeys = &[]containerservice.SSHPublicKey{
			{
				KeyData: &sshKey,
			},
		}
		return o
	}
}

func AgentPool(name, sku string, replicas int32, osDiskSizeGB *int32) specOption {
	return func(o *Spec) *Spec {
		// Check for match against existing pools, modify if found
		for i, val := range *o.internal.AgentPoolProfiles {
			if *val.Name == name {
				(*o.internal.AgentPoolProfiles)[i].VMSize = containerservice.VMSizeTypes(sku)
				(*o.internal.AgentPoolProfiles)[i].OsDiskSizeGB = osDiskSizeGB
				return o
			}
		}

		// No match found, create and append to list of pools
		pool := containerservice.ManagedClusterAgentPoolProfile{
			Name:         &name,
			VMSize:       containerservice.VMSizeTypes(sku),
			OsDiskSizeGB: osDiskSizeGB,
			Count:        &replicas,
			Type:         containerservice.VirtualMachineScaleSets,
		}

		*o.internal.AgentPoolProfiles = append(*o.internal.AgentPoolProfiles, pool)

		return o
	}
}
