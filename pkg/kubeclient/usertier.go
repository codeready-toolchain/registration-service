package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient/resources"
)

type userTierClient struct {
	crtClient
}

type UserTierInterface interface {
	Get(name string) (*crtapi.UserTier, error)
}

// Get returns the UserTier with the specified name, or an error if something went wrong while attempting to retrieve it
func (c *userTierClient) Get(name string) (*crtapi.UserTier, error) {
	result := &crtapi.UserTier{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(resources.UserTierResourcePlural).
		Name(name).
		Do(context.TODO()).
		Into(result)
	return result, err
}
