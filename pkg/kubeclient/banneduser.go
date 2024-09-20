package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bannedUserClient struct {
	crtClient
}

type BannedUserInterface interface {
	ListByEmail(email string) (*crtapi.BannedUserList, error)
	ListByPhoneNumberOrHash(phoneNumberOrHash string) (*crtapi.BannedUserList, error)
}

func (c *bannedUserClient) ListByEmail(email string) (*crtapi.BannedUserList, error) {
	return c.listByLabelForHashedValue(crtapi.BannedUserEmailHashLabelKey, email)
}

// ListByPhoneNumberOrHash will return a list of BannedUsers that have a phone number hash label value matching
// the provided value.  If the value provided is an actual phone number, then the hash will be calculated and then
// used to query the BannedUsers, otherwise if the hash value has been provided, then that value will be used
// directly for the query.
func (c *bannedUserClient) ListByPhoneNumberOrHash(phoneNumberOrHash string) (*crtapi.BannedUserList, error) {
	if md5Matcher.Match([]byte(phoneNumberOrHash)) {
		return c.listByLabel(crtapi.BannedUserPhoneNumberHashLabelKey, phoneNumberOrHash)
	}

	// Default to searching for a hash of the specified value
	return c.listByLabelForHashedValue(crtapi.BannedUserPhoneNumberHashLabelKey, phoneNumberOrHash)
}

// listByLabelForHashedValue returns a BannedUserList containing any BannedUser resources that have a label matching
// the hash of the specified value
func (c *bannedUserClient) listByLabelForHashedValue(labelKey, valueToHash string) (*crtapi.BannedUserList, error) {
	return c.listByLabel(labelKey, hash.EncodeString(valueToHash))
}

// listByLabel returns a BannedUserList containing any BannedUser resources that have a label matching the specified label
func (c *bannedUserClient) listByLabel(labelKey, labelValue string) (*crtapi.BannedUserList, error) {
	bannedUsers := &crtapi.BannedUserList{}
	if err := c.client.List(context.TODO(), bannedUsers, client.InNamespace(c.ns),
		client.MatchingLabels{labelKey: labelValue}); err != nil {
		return nil, err
	}
	return bannedUsers, nil
}
