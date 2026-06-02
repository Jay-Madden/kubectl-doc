package kube

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
)

type Discovery interface {
	ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error)
}

func LoadOverview() (*Overview, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create discovery client: %w", err)
	}
	return LoadOverviewFromDiscovery(discoveryClient)
}

func LoadOverviewFromDiscovery(discoveryClient Discovery) (*Overview, error) {
	_, lists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil && (len(lists) == 0 || !discovery.IsGroupDiscoveryFailedError(err)) {
		return nil, fmt.Errorf("discover API resources: %w", err)
	}
	return BuildOverview(lists)
}
