package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
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
	if err := c.client.Get(context.TODO(), types.NamespacedName{Namespace: c.ns, Name: "toolchain-status"}, result); err != nil {
		return nil, err
	}
	return result, nil
}
