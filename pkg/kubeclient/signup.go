package kubeclient

import (
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
)

const (
	userSignupResourcePlural = "usersignups"
)

type userSignupClient struct {
	crtClient
}

type UserSignupInterface interface {
	Get(name string) (*crtapi.UserSignup, error)
	Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
}

func (c *userSignupClient) Get(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Name(name).
		Do().
		Into(result)
	return result, err
}

func (c *userSignupClient) Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Body(obj).
		Do().
		Into(result)
	return result, err
}
