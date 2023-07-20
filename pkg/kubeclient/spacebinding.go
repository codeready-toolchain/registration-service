package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient/resources"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type spaceBindingClient struct {
	crtClient
}

type SpaceBindingInterface interface {
	ListSpaceBindings(reqs ...labels.Requirement) ([]crtapi.SpaceBinding, error)
}

// List returns the SpaceBindings that match for the provided selector, or an error if something went wrong while attempting to retrieve it
func (c *spaceBindingClient) ListSpaceBindings(reqs ...labels.Requirement) ([]crtapi.SpaceBinding, error) {

	intf, err := dynamic.NewForConfig(&c.cfg)
	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector().Add(reqs...)
	r := schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: resources.SpaceBindingResourcePlural}
	log.Infof(nil, "Listing SpaceBindings with selector: %v", selector.String())
	listOptions := metav1.ListOptions{
		LabelSelector: selector.String(),
	}

	list, err := intf.Resource(r).Namespace(c.ns).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	result := &crtapi.SpaceBindingList{}

	err = c.crtClient.scheme.Convert(list, result, nil)
	return result.Items, err
}
