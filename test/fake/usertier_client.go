package fake

import (
	"encoding/json"
	"testing"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
)

type FakeUserTierClient struct { // nolint:revive
	Tracker    kubetesting.ObjectTracker
	Scheme     *runtime.Scheme
	namespace  string
	MockGet    func(string) (*crtapi.UserTier, error)
	MockCreate func(*crtapi.UserTier) (*crtapi.UserTier, error)
	MockUpdate func(*crtapi.UserTier) (*crtapi.UserTier, error)
	MockDelete func(name string, options *metav1.DeleteOptions) error
}

func NewFakeUserTierClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeUserTierClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.UserTier{}, &crtapi.UserTierList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake MUR client", obj)
	}
	return &FakeUserTierClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeUserTierClient) Get(name string) (*crtapi.UserTier, error) {
	if c.MockGet != nil {
		return c.MockGet(name)
	}

	obj := &crtapi.UserTier{}
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

func (c *FakeUserTierClient) Create(obj *crtapi.UserTier) (*crtapi.UserTier, error) {
	if c.MockCreate != nil {
		return c.MockCreate(obj)
	}

	gvr, err := getGVRFromObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}

	err = c.Tracker.Create(gvr, obj, obj.GetNamespace())
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *FakeUserTierClient) Update(obj *crtapi.UserTier) (*crtapi.UserTier, error) {
	if c.MockUpdate != nil {
		return c.MockUpdate(obj)
	}

	gvr, err := getGVRFromObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}
	err = c.Tracker.Update(gvr, obj, obj.GetNamespace())
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *FakeUserTierClient) Delete(name string, options *metav1.DeleteOptions) error {
	if c.MockDelete != nil {
		return c.MockDelete(name, options)
	}

	gvr, err := getGVRFromObject(&crtapi.UserTier{}, c.Scheme)
	if err != nil {
		return err
	}
	return c.Tracker.Delete(gvr, c.namespace, name)
}
