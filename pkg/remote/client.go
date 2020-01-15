package remote

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type RESTClientGetter struct {
	clientconfig clientcmd.ClientConfig
}

func NewRESTClientGetter(bytes []byte) (*RESTClientGetter, error) {
	clientconfig, err := clientcmd.NewClientConfigFromBytes(bytes)
	if err != nil {
		return nil, err
	}
	return &RESTClientGetter{clientconfig}, nil
}

// ToRESTConfig returns restconfig
func (r *RESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return r.clientconfig.ClientConfig()
}

// ToDiscoveryClient returns discovery client
func (r *RESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	restconfig, err := r.clientconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(restconfig)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

// ToRESTMapper returns a restmapper
func (r *RESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := r.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

// ToRawKubeConfigLoader return kubeconfig loader as-is
func (r *RESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return r.clientconfig
}
