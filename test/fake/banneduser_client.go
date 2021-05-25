package fake

import (
	"crypto/md5"
	"encoding/hex"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
)

type FakeBannedUserClient struct { // nolint: golint
	Tracker               kubetesting.ObjectTracker
	Scheme                *runtime.Scheme
	namespace             string
	MockListByHashedLabel func(labelKey, labelValue string) (*crtapi.BannedUserList, error)
}

func NewFakeBannedUserClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeBannedUserClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.BannedUser{}, &crtapi.BannedUserList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake banneduser client", obj)
	}
	return &FakeBannedUserClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeBannedUserClient) ListByEmail(email string) (*crtapi.BannedUserList, error) {
	return c.listByHashedLabel(crtapi.BannedUserEmailHashLabelKey, email)
}
func (c *FakeBannedUserClient) ListByPhoneNumberOrHash(phone string) (*crtapi.BannedUserList, error) {
	return c.listByHashedLabel(crtapi.BannedUserPhoneNumberHashLabelKey, phone)
}

func (c *FakeBannedUserClient) listByHashedLabel(labelKey, labelValue string) (*crtapi.BannedUserList, error) {
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(labelValue))
	hash := hex.EncodeToString(md5hash.Sum(nil))

	if c.MockListByHashedLabel != nil {
		return c.MockListByHashedLabel(labelKey, labelValue)
	}

	obj := &crtapi.BannedUser{}
	gvr, err := getGVRFromObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}

	gvk, err := apiutil.GVKForObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}

	o, err := c.Tracker.List(gvr, gvk, c.namespace)
	if err != nil {
		return nil, err
	}
	list := o.(*crtapi.BannedUserList)

	objs := []crtapi.BannedUser{}

	for _, bu := range list.Items {
		if bu.Labels[labelKey] == hash {
			objs = append(objs, bu)
		}
	}

	return &crtapi.BannedUserList{
			Items: objs,
		},
		nil
}
