package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
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
	if err := c.client.Get(context.TODO(), types.NamespacedName{Namespace: c.ns, Name: name}, result); err != nil {
		return nil, err
	}
	return result, nil
}
