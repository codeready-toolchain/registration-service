package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient/resources"
)

type toolchainStatusClient struct {
	crtClient
}

type ToolchainStatusInterface interface {
	Get() (*crtapi.ToolchainStatus, error)
}

// Get returns the ToolchainStatus with the "toolchain-status" name, or an error if something went wrong while attempting to retrieve it
// If not found then NotFound error returned
func (c *toolchainStatusClient) Get() (*crtapi.ToolchainStatus, error) {
	result := &crtapi.ToolchainStatus{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(resources.ToolchainStatusPlural).
		Name(resources.ToolchainStatusName).
		Do(context.TODO()).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, err
}
