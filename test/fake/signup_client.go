package fake

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"testing"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type FakeUserSignupClient struct { // nolint: golint
	Tracker               kubetesting.ObjectTracker
	Scheme                *runtime.Scheme
	namespace             string
	MockGet               func(string) (*crtapi.UserSignup, error)
	MockCreate            func(*crtapi.UserSignup) (*crtapi.UserSignup, error)
	MockUpdate            func(*crtapi.UserSignup) (*crtapi.UserSignup, error)
	MockDelete            func(name string, options *v1.DeleteOptions) error
	MockListByHashedLabel func(labelKey, labelValue string) (*crtapi.UserSignupList, error)
}

func NewFakeUserSignupClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeUserSignupClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.UserSignup{}, &crtapi.UserSignupList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake usersignup client", obj)
	}
	return &FakeUserSignupClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeUserSignupClient) Get(name string) (*crtapi.UserSignup, error) {
	if c.MockGet != nil {
		return c.MockGet(name)
	}

	obj := &crtapi.UserSignup{}
	gvr, err := getGVRFromObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}

	o, err := c.Tracker.Get(gvr, c.namespace, name)
	if err != nil {
		return nil, err
	}

	j, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	decoder := scheme.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(j, nil, obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *FakeUserSignupClient) Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	if obj != nil {
		obj.ResourceVersion = uuid.Must(uuid.NewV4()).String()
	}
	if c.MockCreate != nil {
		return c.MockCreate(obj)
	}

	gvr, err := getGVRFromObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	err = c.Tracker.Create(gvr, obj, accessor.GetNamespace())
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *FakeUserSignupClient) Update(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	if c.MockUpdate != nil {
		return c.MockUpdate(obj)
	}

	gvr, err := getGVRFromObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	err = c.Tracker.Update(gvr, obj, accessor.GetNamespace())
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *FakeUserSignupClient) Delete(name string, options *v1.DeleteOptions) error {
	if c.MockDelete != nil {
		return c.MockDelete(name, options)
	}

	gvr, err := getGVRFromObject(&crtapi.UserSignup{}, c.Scheme)
	if err != nil {
		return err
	}
	return c.Tracker.Delete(gvr, c.namespace, name)
}

func (c *FakeUserSignupClient) ListActiveSignupsByPhoneNumberOrHash(phone string) (*crtapi.UserSignupList, error) {
	return c.listByHashedLabel(crtapi.UserSignupUserPhoneHashLabelKey, phone)
}

func (c *FakeUserSignupClient) listByHashedLabel(labelKey, labelValue string) (*crtapi.UserSignupList, error) {
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(labelValue))
	hash := hex.EncodeToString(md5hash.Sum(nil))

	if c.MockListByHashedLabel != nil {
		return c.MockListByHashedLabel(labelValue, labelKey)
	}

	obj := &crtapi.UserSignup{}
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
	list := o.(*crtapi.UserSignupList)

	objs := []crtapi.UserSignup{}

	for _, bu := range list.Items {
		if bu.Labels[labelKey] == hash {
			objs = append(objs, bu)
		}
	}

	return &crtapi.UserSignupList{
			Items: objs,
		},
		nil
}
