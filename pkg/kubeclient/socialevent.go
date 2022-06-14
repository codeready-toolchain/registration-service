package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
)

const (
	socialeventResourcePlural = "socialevents"
)

type socialeventClient struct {
	crtClient
}

type SocialEventInterface interface {
	Get(name string) (*crtapi.SocialEvent, error)
}

// Get returns the SocialEvent with the specified name, or an error if something went wrong while attempting to retrieve it
func (c *socialeventClient) Get(name string) (*crtapi.SocialEvent, error) {
	result := &crtapi.SocialEvent{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(socialeventResourcePlural).
		Name(name).
		Do(context.TODO()).
		Into(result)
	return result, err
}
