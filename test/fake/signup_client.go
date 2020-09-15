package fake

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
)

type FakeUserSignupClient struct {
	Tracker         testing.ObjectTracker
	Scheme          *runtime.Scheme
	namespace       string
	MockGet         func(string) (*crtapi.UserSignup, error)
	MockCreate      func(*crtapi.UserSignup) (*crtapi.UserSignup, error)
	MockUpdate      func(*crtapi.UserSignup) (*crtapi.UserSignup, error)
	MockDelete      func(name string, options *v1.DeleteOptions) error
	MockListByValue func(value, label string) (*crtapi.UserSignupList, error)
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
			panic("could not add object to tracker: " + err.Error())
		}
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

func (c *FakeUserSignupClient) ListByValue(value, label string) (*crtapi.UserSignupList, error) {
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(value))
	hash := hex.EncodeToString(md5hash.Sum(nil))

	if c.MockListByValue != nil {
		return c.MockListByValue(value, label)
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
		if bu.Labels[label] == hash {
			objs = append(objs, bu)
		}
	}

	return &crtapi.UserSignupList{
			Items: objs,
		},
		nil
}
