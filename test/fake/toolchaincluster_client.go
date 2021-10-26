package fake

import (
	"encoding/json"
	"testing"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
)

type FakeToolchainClusterClient struct { // nolint: golint
	Tracker   kubetesting.ObjectTracker
	Scheme    *runtime.Scheme
	namespace string
	MockGet   func(string) (*crtapi.ToolchainCluster, error)
}

func NewFakeToolchainClusterClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeToolchainClusterClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.ToolchainCluster{}, &crtapi.ToolchainClusterList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake toolchain cluster client", obj)
	}
	return &FakeToolchainClusterClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeToolchainClusterClient) Get(name string) (*crtapi.ToolchainCluster, error) {
	if c.MockGet != nil {
		return c.MockGet(name)
	}

	obj := &crtapi.ToolchainCluster{}
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
