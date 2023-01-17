package kubeclient

import (
	"context"
	"fmt"
	"regexp"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	md5Matcher = regexp.MustCompile("(?i)[a-f0-9]{32}$")
)

type userSignupClient struct {
	crtClient
}

// UserSignupInterface is the interface for user signup.
type UserSignupInterface interface {
	Get(name string) (*crtapi.UserSignup, error)
	Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	Update(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	ListActiveSignupsByPhoneNumberOrHash(phoneNumberOrHash string) (*crtapi.UserSignupList, error)
}

// Get returns the UserSignup with the specified name, or an error if something went wrong while attempting to retrieve it
// If not found then NotFound error returned
func (c *userSignupClient) Get(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(UserSignupResourcePlural).
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
		Resource(UserSignupResourcePlural).
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
		Resource(UserSignupResourcePlural).
		Name(obj.Name).
		Body(obj).
		Do(context.TODO()).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListActiveSignupsByPhoneNumberOrHash will return a list of non-deactivated UserSignups that have a phone number hash
// label value matching the provided value.  If the value provided is an actual phone number, then the hash will be
// calculated and then used to query the UserSignups, otherwise if the hash value has been provided, then that value
// will be used directly for the query.
func (c *userSignupClient) ListActiveSignupsByPhoneNumberOrHash(phoneNumberOrHash string) (*crtapi.UserSignupList, error) {
	if md5Matcher.Match([]byte(phoneNumberOrHash)) {
		return c.listActiveSignupsByLabel(crtapi.BannedUserPhoneNumberHashLabelKey, phoneNumberOrHash)
	}

	// Default to searching for a hash of the specified value
	return c.listActiveSignupsByLabelForHashedValue(crtapi.BannedUserPhoneNumberHashLabelKey, phoneNumberOrHash)
}

// listActiveSignupsByLabelForHashedValue returns a UserSignupList containing any non-deactivated UserSignup resources
// that have a label matching the md5 hash of the specified value
func (c *userSignupClient) listActiveSignupsByLabelForHashedValue(labelKey, valueToHash string) (*crtapi.UserSignupList, error) {
	return c.listActiveSignupsByLabel(labelKey, hash.EncodeString(valueToHash))
}

// listActiveSignupsByLabel returns a UserSignupList containing any non-deactivated UserSignup resources that have a
// label matching the specified label
func (c *userSignupClient) listActiveSignupsByLabel(labelKey, labelValue string) (*crtapi.UserSignupList, error) {
	intf, err := dynamic.NewForConfig(&c.cfg)
	if err != nil {
		return nil, err
	}

	r := schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: UserSignupResourcePlural}
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s!=%s,%s=%s", crtapi.UserSignupStateLabelKey, crtapi.UserSignupStateLabelValueDeactivated, labelKey, labelValue),
	}

	list, err := intf.Resource(r).Namespace(c.ns).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	result := &crtapi.UserSignupList{}

	err = c.crtClient.scheme.Convert(list, result, nil)
	return result, err
}
