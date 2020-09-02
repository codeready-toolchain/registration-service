package kubeclient

import (
	"github.com/gin-gonic/gin"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
)

const (
	userSignupResourcePlural = "usersignups"
)

type userSignupClient struct {
	crtClient
}

// UserSignupInterface is the interface for user signup.
type UserSignupInterface interface {
	Get(ctx *gin.Context, name string) (*crtapi.UserSignup, error)
	Create(ctx *gin.Context, obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	Update(ctx *gin.Context, obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
}

// Get returns the UserSignup with the specified name, or an error if something went wrong while attempting to retrieve it
// If not found then NotFound error returned
func (c *userSignupClient) Get(ctx *gin.Context, name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Name(name).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, err
}

// Create creates a new UserSignup resource in the cluster, and returns the resulting UserSignup that was created, or
// an error if something went wrong
func (c *userSignupClient) Create(ctx *gin.Context, obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Body(obj).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, err
}

// Update will update an existing UserSignup resource in the cluster, returning an error if something went wrong
func (c *userSignupClient) Update(ctx *gin.Context, obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Put().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Name(obj.Name).
		Body(obj).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
