package kubeclient

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"k8s.io/client-go/kubernetes/scheme"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	bannedUserResourcePlural = "bannedusers"
)

type bannedUserClient struct {
	crtClient
}

type BannedUserInterface interface {
	List(email string) (*crtapi.BannedUserList, error)
}

// List returns a BannedUserList containing any BannedUser resources that have a label matching the specified email address
func (c *bannedUserClient) List(email string) (*crtapi.BannedUserList, error) {
	result := &crtapi.BannedUserList{}

	// Calculate the md5 hash for the email
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(email))
	emailHash := hex.EncodeToString(md5hash.Sum(nil))

	listOptions := v1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", crtapi.BannedUserEmailHashLabelKey, emailHash)}

	err := c.client.
		Get().
		Namespace(c.ns).
		Resource(bannedUserResourcePlural).
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Do().
		Into(result)
	return result, err
}
