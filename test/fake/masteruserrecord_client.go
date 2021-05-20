package fake

import (
	"encoding/json"
	"testing"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
)

type FakeMasterUserRecordClient struct { // nolint: golint
	Tracker    kubetesting.ObjectTracker
	Scheme     *runtime.Scheme
	namespace  string
	MockGet    func(string) (*crtapi.MasterUserRecord, error)
	MockCreate func(*crtapi.MasterUserRecord) (*crtapi.MasterUserRecord, error)
	MockUpdate func(*crtapi.MasterUserRecord) (*crtapi.MasterUserRecord, error)
	MockDelete func(name string, options *v1.DeleteOptions) error
}

func NewFakeMasterUserRecordClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeMasterUserRecordClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.MasterUserRecord{}, &crtapi.MasterUserRecordList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake MUR client", obj)
	}
	return &FakeMasterUserRecordClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeMasterUserRecordClient) Get(name string) (*crtapi.MasterUserRecord, error) {
	if c.MockGet != nil {
		return c.MockGet(name)
	}

	obj := &crtapi.MasterUserRecord{}
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

func (c *FakeMasterUserRecordClient) Create(obj *crtapi.MasterUserRecord) (*crtapi.MasterUserRecord, error) {
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

func (c *FakeMasterUserRecordClient) Update(obj *crtapi.MasterUserRecord) (*crtapi.MasterUserRecord, error) {
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

func (c *FakeMasterUserRecordClient) Delete(name string, options *v1.DeleteOptions) error {
	if c.MockDelete != nil {
		return c.MockDelete(name, options)
	}

	gvr, err := getGVRFromObject(&crtapi.MasterUserRecord{}, c.Scheme)
	if err != nil {
		return err
	}
	return c.Tracker.Delete(gvr, c.namespace, name)
}
