package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
)

const (
	toolchainClusterResourcePlural = "toolchainclusters"
)

type toolchainClusterClient struct {
	crtClient
}

// ToolchainClusterInterface is the interface for toolchain clusters.
type ToolchainClusterInterface interface {
	List() ([]crtapi.ToolchainCluster, error)
}

// List returns the Toolchain Clusters or an error if something went wrong while attempting to retrieve it
func (c *toolchainClusterClient) List() ([]crtapi.ToolchainCluster, error) {
	result := &crtapi.ToolchainClusterList{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(toolchainClusterResourcePlural).
		Do(context.TODO()).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result.Items, err
}
