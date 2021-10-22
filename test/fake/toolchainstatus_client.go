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

type FakeToolchainStatusClient struct { // nolint: golint
	Tracker   kubetesting.ObjectTracker
	Scheme    *runtime.Scheme
	namespace string
	MockGet   func() (*crtapi.ToolchainStatus, error)
}

func NewFakeToolchainStatusClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeToolchainStatusClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.ToolchainStatus{}, &crtapi.ToolchainStatusList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake toolchainstatus client", obj)
	}
	return &FakeToolchainStatusClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeToolchainStatusClient) Get() (*crtapi.ToolchainStatus, error) {
	if c.MockGet != nil {
		return c.MockGet()
	}

	obj := &crtapi.ToolchainStatus{}
	gvr, err := getGVRFromObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}

	o, err := c.Tracker.Get(gvr, c.namespace, "toolchain-status")
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
