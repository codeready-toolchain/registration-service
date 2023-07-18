package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient/resources"
)

type spaceClient struct {
	crtClient
}

type SpaceInterface interface {
	Get(name string) (*crtapi.Space, error)
}

// List returns the Spaces that match for the provided selector, or an error if something went wrong while attempting to retrieve it
func (c *spaceClient) Get(name string) (*crtapi.Space, error) {
	result := &crtapi.Space{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(resources.SpaceResourcePlural).
		Name(name).
		Do(context.TODO()).
		Into(result)
	return result, err
}
