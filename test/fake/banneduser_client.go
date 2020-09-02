package fake

import (
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
)

type FakeBannedUserClient struct {
	Tracker   testing.ObjectTracker
	Scheme    *runtime.Scheme
	namespace string
	MockList  func(string) (*crtapi.BannedUserList, error)
}

func NewFakeBannedUserClient(namespace string, initObjs ...runtime.Object) *FakeBannedUserClient {
	clientScheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(clientScheme)
	if err != nil {
		log.Error(err, "Error adding to scheme")
		os.Exit(1)
	}
	crtapi.SchemeBuilder.Register(&crtapi.BannedUser{}, &crtapi.BannedUserList{})

	tracker := testing.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	for _, obj := range initObjs {
		err := tracker.Add(obj)
		if err != nil {
			log.Error(err, "failed to add object to fake banned user client", "object", obj)
			panic("could not add object to tracker: " + err.Error())
		}
	}
	return &FakeBannedUserClient{
		Tracker:   tracker,
		Scheme:    clientScheme,
		namespace: namespace,
	}
}

func (c *FakeBannedUserClient) List(email string) (*crtapi.BannedUserList, error) {
	if c.MockList != nil {
		return c.MockList(email)
	}

	obj := &crtapi.BannedUser{}
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
	list := o.(*crtapi.BannedUserList)

	objs := []crtapi.BannedUser{}

	for _, bu := range list.Items {
		if bu.Spec.Email == email {
			objs = append(objs, bu)
		}
	}

	return &crtapi.BannedUserList{
			Items: objs,
		},
		nil
}
