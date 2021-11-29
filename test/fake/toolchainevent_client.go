package fake

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
)

type FakeToolchainEventClient struct { // nolint: golint
	Tracker                  kubetesting.ObjectTracker
	Scheme                   *runtime.Scheme
	namespace                string
	MockUpdate               func(*crtapi.ToolchainEvent) (*crtapi.ToolchainEvent, error)
	MockListByActivationCode func(string) (*crtapi.ToolchainEventList, error)
}

func NewFakeToolchainEventClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeToolchainEventClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.ToolchainEvent{}, &crtapi.ToolchainEventList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake toolchainevent client", obj)
	}
	return &FakeToolchainEventClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeToolchainEventClient) Update(obj *crtapi.ToolchainEvent) (*crtapi.ToolchainEvent, error) {
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

func (c *FakeToolchainEventClient) ListByActivationCode(activationCode string) (*crtapi.ToolchainEventList, error) {
	if c.MockListByActivationCode != nil {
		return c.MockListByActivationCode(activationCode)
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
	list := o.(*crtapi.ToolchainEventList)

	objs := []crtapi.ToolchainEvent{}

	for _, bu := range list.Items {
		if bu.Labels[crtapi.ToolchainEventActivationCodeLabelKey] == activationCode {
			objs = append(objs, bu)
		}
	}

	return &crtapi.ToolchainEventList{
			Items: objs,
		},
		nil
}
