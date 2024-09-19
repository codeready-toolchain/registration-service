package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
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
	if err := c.client.Get(context.TODO(), types.NamespacedName{Namespace: c.ns, Name: name}, result); err != nil {
		return nil, err
	}
	return result, nil
}
