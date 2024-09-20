package kubeclient

import (
	"context"
	"regexp"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	ListActiveSignupsByPhoneNumberOrHash(phoneNumberOrHash string) ([]*crtapi.UserSignup, error)
}

// Get returns the UserSignup with the specified name, or an error if something went wrong while attempting to retrieve it
// If not found then NotFound error returned
func (c *userSignupClient) Get(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	if err := c.client.Get(context.TODO(), types.NamespacedName{Namespace: c.ns, Name: name}, result); err != nil {
		return nil, err
	}
	return result, nil
}

// Create creates a new UserSignup resource in the cluster, and returns the resulting UserSignup that was created, or
// an error if something went wrong
func (c *userSignupClient) Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	if err := c.client.Create(context.TODO(), obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// Update will update an existing UserSignup resource in the cluster, returning an error if something went wrong
func (c *userSignupClient) Update(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	err := c.client.Update(context.TODO(), obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// ListActiveSignupsByPhoneNumberOrHash will return a list of non-deactivated UserSignups that have a phone number hash
// label value matching the provided value.  If the value provided is an actual phone number, then the hash will be
// calculated and then used to query the UserSignups, otherwise if the hash value has been provided, then that value
// will be used directly for the query.
func (c *userSignupClient) ListActiveSignupsByPhoneNumberOrHash(phoneNumberOrHash string) ([]*crtapi.UserSignup, error) {
	if md5Matcher.Match([]byte(phoneNumberOrHash)) {
		return c.listActiveSignupsByLabel(crtapi.BannedUserPhoneNumberHashLabelKey, phoneNumberOrHash)
	}

	// Default to searching for a hash of the specified value
	return c.listActiveSignupsByLabelForHashedValue(crtapi.BannedUserPhoneNumberHashLabelKey, phoneNumberOrHash)
}

// listActiveSignupsByLabelForHashedValue returns an array of UserSignups containing any non-deactivated UserSignup resources
// that have a label matching the md5 hash of the specified value
func (c *userSignupClient) listActiveSignupsByLabelForHashedValue(labelKey, valueToHash string) ([]*crtapi.UserSignup, error) {
	return c.listActiveSignupsByLabel(labelKey, hash.EncodeString(valueToHash))
}

// listActiveSignupsByLabel returns an array of UserSignups containing any non-deactivated UserSignup resources that have a
// label matching the specified label
func (c *userSignupClient) listActiveSignupsByLabel(labelKey, labelValue string) ([]*crtapi.UserSignup, error) {

	// TODO add unit tests checking that the label selection works as expected. Right now, it's not possible to do that thanks to the great abstraction and multiple level of layers, mocks, and services.
	selector := labels.NewSelector()
	stateRequirement, err := labels.NewRequirement(crtapi.UserSignupStateLabelKey, selection.NotIn, []string{crtapi.UserSignupStateLabelValueDeactivated, crtapi.UserSignupStateLabelValueNotReady})
	if err != nil {
		return nil, err
	}
	selector = selector.Add(*stateRequirement)
	labelRequirement, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelValue})
	if err != nil {
		return nil, err
	}
	selector = selector.Add(*labelRequirement)

	userSignups := &crtapi.UserSignupList{}
	err = c.client.List(context.TODO(), userSignups, client.InNamespace(c.ns), client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return nil, err
	}

	result := make([]*crtapi.UserSignup, len(userSignups.Items))

	for i := range userSignups.Items {
		result[i] = &userSignups.Items[i]
	}

	return result, err
}
