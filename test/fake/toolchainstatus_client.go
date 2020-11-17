package fake

import (
	"encoding/json"
	"os"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
)

type FakeToolchainStatusClient struct {
	Tracker   testing.ObjectTracker
	Scheme    *runtime.Scheme
	namespace string
	MockGet   func() (*crtapi.ToolchainStatus, error)
}

func NewFakeToolchainStatusClient(namespace string, initObjs ...runtime.Object) *FakeToolchainStatusClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	if err != nil {
		log.Error(err, "Error adding to scheme")
		os.Exit(1)
	}
	crtapi.SchemeBuilder.Register(&crtapi.ToolchainStatus{}, &crtapi.ToolchainStatusList{})

	tracker := testing.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		if err != nil {
			log.Error(err, "failed to add object to fake toolchainstatus client", "object", obj)
			panic("could not add object to tracker: " + err.Error())
		}
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
