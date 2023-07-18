package fake

import (
	"testing"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
)

type FakeSpaceBindingClient struct { // nolint:revive
	Tracker   kubetesting.ObjectTracker
	Scheme    *runtime.Scheme
	namespace string
	MockList  func(reqs ...labels.Requirement) ([]crtapi.SpaceBinding, error)
}

var _ kubeclient.SpaceBindingInterface = &FakeSpaceBindingClient{}

func NewFakeSpaceBindingClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeSpaceBindingClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.SpaceBinding{}, &crtapi.SpaceBindingList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake spacebinding client", obj)
	}
	return &FakeSpaceBindingClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeSpaceBindingClient) ListSpaceBindings(reqs ...labels.Requirement) ([]crtapi.SpaceBinding, error) {

	if c.MockList != nil {
		return c.MockList(reqs...)
	}

	obj := &crtapi.SpaceBinding{}
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
	list := o.(*crtapi.SpaceBindingList)

	return list.Items, nil
}
