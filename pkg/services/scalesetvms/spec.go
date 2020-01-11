package scalesetvms

type specOption func(*Spec) *Spec

type Spec struct {
	subscriptionID string
	group          string
	vmss           string
	instance       string
}
