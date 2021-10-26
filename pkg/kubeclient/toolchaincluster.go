package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
)

const (
	toolchainClusterResourcePlural = "toolchainclusters"
)

type toolchainClusterClient struct {
	crtClient
}

// ToolchainClusterInterface is the interface for toolchain clusters.
type ToolchainClusterInterface interface {
	Get(name string) (*crtapi.ToolchainCluster, error)
}

// Get returns the ToolchainCluster with the specified name, or an error if something went wrong while attempting to retrieve it
// If not found then NotFound error returned
func (c *toolchainClusterClient) Get(name string) (*crtapi.ToolchainCluster, error) {
	result := &crtapi.ToolchainCluster{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(toolchainClusterResourcePlural).
		Name(name).
		Do(context.TODO()).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, err
}
