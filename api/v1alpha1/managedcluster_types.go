package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManagedClusterSpec defines the desired state of ManagedCluster
type ManagedClusterSpec struct {
	// SubscriptionID is the subscription id for an azure resource.
	// +kubebuilder:validation:Pattern=`^[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}$`
	SubscriptionID string `json:"subscriptionId"`
	// ResourceGroup is the resource group name for an azure resource.
	// +kubebuilder:validation:Pattern=`^[-\w\._\(\)]+$`
	ResourceGroup string `json:"resourceGroup"`
	// Location is the region where the azure resource resides.
	Location string `json:"location"`
	// Name is the name of the managed cluster in Azure.
	Name string `json:"name"`
	// CredentialsRef is a reference to the service principal credentials which will be placed on the cluster.
	CredentialsRef corev1.SecretReference `json:"credentialsRef"`
	// KubeconfigRef is a reference to the service principal credentials which will be placed on the cluster.
	KubeconfigRef *corev1.SecretKeySelector `json:"kubeconfigRef,omitempty"`
	// Kustomizations is an array of kustomize remote targets to apply to the cluster
	Kustomizations []string `json:"kustomizations,omitempty"`
	// SSHPublicKey is a string literal containing an ssh public key.
	SSHPublicKey string `json:"sshPublicKey"`
	// Version defines the kubernetes version of the cluster.
	Version string `json:"version"`
	// AgentPools is the list of additional node pools managed by this cluster.
	// +kubebuilder:validation:MinItems=1
	AgentPools []AgentPoolTemplate `json:"nodePools"`
	// LoadBalancerSKU for the managed cluster. Possible values include: 'Standard', 'Basic'. Defaults to standard.
	// +kubebuilder:validation:Enum=Standard;Basic
	LoadBalancerSKU *string `json:"loadBalancerSku,omitempty"`
	// NetworkPlugin used for building Kubernetes network. Possible values include: 'Azure', 'Kubenet'. Defaults to Azure.
	// +kubebuilder:validation:Enum=Azure;Kubenet
	NetworkPlugin *string `json:"networkPlugin,omitempty"`
	// NetworkPolicy used for building Kubernetes network. Possible values include: 'NetworkPolicyCalico', 'NetworkPolicyAzure'
	// +kubebuilder:validation:Enum=NetworkPolicyCalico;NetworkPolicyAzure
	NetworkPolicy *string `json:"networkPolicy,omitempty"`
	// PodCIDR is a CIDR notation IP range from which to assign pod IPs when kubenet is used.
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}(\/([0-9]|[1-2][0-9]|3[0-2]))?$`
	PodCIDR *string `json:"podCidr,omitempty"`
	// ServiceCIDR is a CIDR notation IP range from which to assign service cluster IPs. It must not overlap with any Subnet IP ranges.
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}(\/([0-9]|[1-2][0-9]|3[0-2]))?$`
	ServiceCIDR *string `json:"serviceCidr,omitempty"`
	// DNSServiceIP - An IP address assigned to the Kubernetes DNS service. It must be within the Kubernetes service address range specified in serviceCidr.
	DNSServiceIP *string `json:"dnsServiceIP,omitempty"`
}

// ManagedClusterStatus defines the observed state of ManagedCluster
type ManagedClusterStatus struct {
	Future []byte `json:"future,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ManagedCluster is the Schema for the azuremanagedclusters API
type ManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedClusterSpec   `json:"spec,omitempty"`
	Status ManagedClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedClusterList contains a list of ManagedCluster
type ManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedCluster{}, &ManagedClusterList{})
}
