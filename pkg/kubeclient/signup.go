package kubeclient

import (
	"encoding/json"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	userSignupResourcePlural = "usersignups"
)

type userSignupClient struct {
	crtClient
}

// UserSignupInterface is the interface for user signup.
type UserSignupInterface interface {
	Get(name string) (*crtapi.UserSignup, error)
	Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	Patch(name string, obj map[string]string) (*crtapi.UserSignup, error)
}

// Get returns the UserSignup with the specified name, or an error if something went wrong while attempting to retrieve it
// If not found then NotFound error returned
func (c *userSignupClient) Get(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Name(name).
		Do().
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, err
}

// Create creates a new UserSignup resource in the cluster, and returns the resulting UserSignup that was created, or
// an error if something went wrong
func (c *userSignupClient) Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Body(obj).
		Do().
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, err
}

// Patch updates a new UserSignup resource in the cluster, and returns the resulting UserSignup that was created, or
// an error if something went wrong
func (c *userSignupClient) Patch(name string, obj map[string]string) (*crtapi.UserSignup, error) {
	//result := &crtapi.UserSignup{}
	//err := c.client.Put().
	//	Namespace(c.ns).
	//	Resource(userSignupResourcePlural).
	//	Body(obj).
	//	Do().
	//	Into(result)
	//if err != nil {
	//	return nil, err
	//}
	//return result, err

	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	result := &crtapi.UserSignup{}
	err = c.client.Patch(types.JSONPatchType).
		Namespace(c.ns).
		Name(name).
		Resource(userSignupResourcePlural).
		Body(data).
		Do().
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, err
}
