package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient/resources"
)

type masterUserRecordClient struct {
	crtClient
}

type MasterUserRecordInterface interface {
	Get(name string) (*crtapi.MasterUserRecord, error)
}

// Get returns the MasterUserRecord with the specified name, or an error if something went wrong while attempting to retrieve it
func (c *masterUserRecordClient) Get(name string) (*crtapi.MasterUserRecord, error) {
	result := &crtapi.MasterUserRecord{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(resources.MurResourcePlural).
		Name(name).
		Do(context.TODO()).
		Into(result)
	return result, err
}
