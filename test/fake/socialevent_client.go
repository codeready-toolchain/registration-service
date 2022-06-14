package fake

import (
	"encoding/json"
	"testing"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
)

type FakeSocialEventClient struct { // nolint:revive
	Tracker               kubetesting.ObjectTracker
	Scheme                *runtime.Scheme
	namespace             string
	MockGet               func(string) (*crtapi.SocialEvent, error)
	MockCreate            func(*crtapi.SocialEvent) (*crtapi.SocialEvent, error)
	MockUpdate            func(*crtapi.SocialEvent) (*crtapi.SocialEvent, error)
	MockDelete            func(name string, options *metav1.DeleteOptions) error
	MockListByHashedLabel func(labelKey, labelValue string) (*crtapi.SocialEventList, error)
}

var _ kubeclient.SocialEventInterface = &FakeSocialEventClient{}

func NewFakeSocialEventClient(t *testing.T, namespace string, initObjs ...runtime.Object) *FakeSocialEventClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	require.NoError(t, err, "Error adding to scheme")
	crtapi.SchemeBuilder.Register(&crtapi.SocialEvent{}, &crtapi.SocialEventList{})

	tracker := kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		require.NoError(t, err, "failed to add object %v to fake usersignup client", obj)
	}
	return &FakeSocialEventClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeSocialEventClient) Get(name string) (*crtapi.SocialEvent, error) {
	if c.MockGet != nil {
		return c.MockGet(name)
	}

	obj := &crtapi.SocialEvent{}
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
