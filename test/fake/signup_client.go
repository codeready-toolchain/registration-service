package fake

import (
	"encoding/json"
	"os"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
)

type FakeUserSignupClient struct {
	Tracker   testing.ObjectTracker
	Scheme    *runtime.Scheme
	namespace string
}

func NewFakeUserSignupClient(namespace string, initObjs ...runtime.Object) *FakeUserSignupClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	if err != nil {
		log.Error(err, "Error adding to scheme")
		os.Exit(1)
	}
	crtapi.SchemeBuilder.Register(&crtapi.UserSignup{}, &crtapi.UserSignupList{})

	tracker := testing.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		if err != nil {
			log.Error(err, "failed to add object to fake user signup client", "object", obj)
			os.Exit(1)
			return nil
		}
	}
	return &FakeUserSignupClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeUserSignupClient) Get(name string) (*crtapi.UserSignup, error) {
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
	gvr, err := getGVRFromObject(&crtapi.UserSignup{}, c.Scheme)
	if err != nil {
		return err
	}
	return c.Tracker.Delete(gvr, c.namespace, name)
}
