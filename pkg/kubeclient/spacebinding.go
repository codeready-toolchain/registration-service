package kubeclient

import (
	"context"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type spaceBindingClient struct {
	crtClient
}

type SpaceBindingInterface interface {
	ListSpaceBindings(reqs ...labels.Requirement) ([]crtapi.SpaceBinding, error)
}

// List returns the SpaceBindings that match for the provided selector, or an error if something went wrong while attempting to retrieve it
func (c *spaceBindingClient) ListSpaceBindings(reqs ...labels.Requirement) ([]crtapi.SpaceBinding, error) {

	selector := labels.NewSelector().Add(reqs...)

	result := &crtapi.SpaceBindingList{}
	err := c.client.List(context.TODO(), result, client.InNamespace(c.ns),
		client.MatchingLabelsSelector{Selector: selector})

	return result.Items, err
}
