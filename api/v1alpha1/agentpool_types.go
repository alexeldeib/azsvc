//go:generate easytags $GOFILE json:camel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentPoolSpec defines the desired state of AgentPool
type AgentPoolSpec struct {
	// SubscriptionID is the subscription id for an Azure resource.
	// +kubebuilder:validation:Pattern=`^[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}$`
	SubscriptionID string `json:"subscriptionId"`
	// ResourceGroup is the resource group name for an Azure resource.
	// +kubebuilder:validation:Pattern=`^[-\w\._\(\)]+$`
	ResourceGroup string `json:"resourceGroup"`
	// Cluster is the name of the AKS cluster to which this agent pool is attached.
	Cluster           string `json:"cluster"`
	AgentPoolTemplate `json:"-"`
}

type AgentPoolTemplate struct {
	// Name is the name of the node pool.
	Name string `json:"name"`
	// SKU is the size of the VMs in the node pool.
	SKU string `json:"sku"`
	// Replicas is the number of nodes in this agent pool.
	Replicas int32 `json:"replicas"`
	// Version defines the kubernetes version of the agent pool.
	Version *string `json:"version,omitempty"`
	// OSDiskSizeGB is the disk size for every machine in this master/agent pool. If you specify 0, it will apply the default osDisk size according to the vmSize specified.
	OSDiskSizeGB *int32 `json:"osDiskSizeGB,omitempty"`
}

// AgentPoolStatus defines the observed state of AgentPool
type AgentPoolStatus struct {
	ID                *string `json:"id,omitempty"`
	ProvisioningState *string `json:"provisioningState,omitempty"`
	Future            []byte  `json:"future,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AgentPool is the Schema for the agentpools API
type AgentPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentPoolSpec   `json:"spec,omitempty"`
	Status AgentPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentPoolList contains a list of AgentPool
type AgentPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentPool{}, &AgentPoolList{})
}
