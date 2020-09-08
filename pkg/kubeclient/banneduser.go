package kubeclient

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	bannedUserResourcePlural = "bannedusers"
)

type bannedUserClient struct {
	crtClient
}

type BannedUserInterface interface {
	ListByEmail(email string) (*crtapi.BannedUserList, error)
	ListByPhoneNumber(phoneNumber string) (*crtapi.BannedUserList, error)
}

// ListByEmail returns a BannedUserList containing any BannedUser resources that have a label matching the specified email address
func (c *bannedUserClient) ListByEmail(email string) (*crtapi.BannedUserList, error) {

	// Calculate the md5 hash for the email
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(email))
	emailHash := hex.EncodeToString(md5hash.Sum(nil))

	intf, err := dynamic.NewForConfig(&c.cfg)
	if err != nil {
		return nil, err
	}

	r := schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: bannedUserResourcePlural}
	listOptions := v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", crtapi.BannedUserEmailHashLabelKey, emailHash),
	}

	list, err := intf.Resource(r).Namespace(c.ns).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	result := &crtapi.BannedUserList{}

	err = c.crtClient.scheme.Convert(list, result, nil)
	return result, err
}

// ListByPhoneNumber returns a BannedUserList containing any BannedUser resources that have a label matching the specified phone address
func (c *bannedUserClient) ListByPhoneNumber(phoneNumber string) (*crtapi.BannedUserList, error) {

	// Calculate the md5 hash for the phoneNumber
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(phoneNumber))
	phoneHash := hex.EncodeToString(md5hash.Sum(nil))

	intf, err := dynamic.NewForConfig(&c.cfg)
	if err != nil {
		return nil, err
	}

	r := schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: bannedUserResourcePlural}
	listOptions := v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", crtapi.BannedUserPhoneNumberHashLabelKey, phoneHash),
	}

	list, err := intf.Resource(r).Namespace(c.ns).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	result := &crtapi.BannedUserList{}

	err = c.crtClient.scheme.Convert(list, result, nil)
	return result, err
}
