package kubeclient

import (
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/gin-gonic/gin"
)

const (
	masterUserRecordResourcePlural = "masteruserrecords"
)

type masterUserRecordClient struct {
	crtClient
}

type MasterUserRecordInterface interface {
	Get(ctx *gin.Context, name string) (*crtapi.MasterUserRecord, error)
}

// Get returns the MasterUserRecord with the specified name, or an error if something went wrong while attempting to retrieve it
func (c *masterUserRecordClient) Get(ctx *gin.Context, name string) (*crtapi.MasterUserRecord, error) {
	result := &crtapi.MasterUserRecord{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(masterUserRecordResourcePlural).
		Name(name).
		Do(ctx).
		Into(result)
	return result, err
}
