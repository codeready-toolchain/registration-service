package kubeclient

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

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
	Get(name string) (*crtapi.UserSignup, error)
	Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	Update(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	ListByPhoneNumber(phoneNumber string) (*crtapi.UserSignupList, error)
}

// Get returns the UserSignup with the specified name, or an error if something went wrong while attempting to retrieve it
// If not found then NotFound error returned
func (c *userSignupClient) Get(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Name(name).
		Do(context.TODO()).
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
		Do(context.TODO()).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, err
}

// Update will update an existing UserSignup resource in the cluster, returning an error if something went wrong
func (c *userSignupClient) Update(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Put().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Name(obj.Name).
		Body(obj).
		Do(context.TODO()).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListByPhoneNumber returns a UserSignupList containing any UserSignup resources that have a label matching the specified phone number
func (c *userSignupClient) ListByPhoneNumber(phoneNumber string) (*crtapi.UserSignupList, error) {

	// Calculate the md5 hash for the phoneNumber
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(phoneNumber))
	phoneHash := hex.EncodeToString(md5hash.Sum(nil))

	intf, err := dynamic.NewForConfig(&c.cfg)
	if err != nil {
		return nil, err
	}

	r := schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: userSignupResourcePlural}
	listOptions := v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", crtapi.UserSignupUserPhoneHashLabelKey, phoneHash),
	}

	list, err := intf.Resource(r).Namespace(c.ns).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	result := &crtapi.UserSignupList{}

	err = c.crtClient.scheme.Convert(list, result, nil)
	return result, err
}
