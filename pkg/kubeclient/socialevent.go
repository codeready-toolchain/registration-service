package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
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
	if err := c.client.Get(context.TODO(), types.NamespacedName{Namespace: c.ns, Name: name}, result); err != nil {
		return nil, err
	}
	return result, nil
}
