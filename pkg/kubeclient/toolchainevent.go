package kubeclient

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
)

const (
	toolchainEventResourcePlural = "toolchainevents"
)

type toolchainEventClient struct {
	crtClient
}

// ToolchainEventInterface is the interface for toolchain events.
type ToolchainEventInterface interface {
	Update(obj *crtapi.ToolchainEvent) (*crtapi.ToolchainEvent, error)
	ListByActivationCode(activationCode string) (*crtapi.ToolchainEventList, error)
}

// Update will update an existing ToolchainEvent resource in the cluster, returning an error if something went wrong
func (c *toolchainEventClient) Update(obj *crtapi.ToolchainEvent) (*crtapi.ToolchainEvent, error) {
	result := &crtapi.ToolchainEvent{}
	err := c.client.Put().
		Namespace(c.ns).
		Resource(toolchainEventResourcePlural).
		Name(obj.Name).
		Body(obj).
		Do(context.TODO()).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListByActivationCode returns all ToolchainEvent resources with the specified activation code, or an error if something went
// wrong while attempting to retrieve them.
func (c *toolchainEventClient) ListByActivationCode(activationCode string) (*crtapi.ToolchainEventList, error) {

	intf, err := dynamic.NewForConfig(&c.cfg)
	if err != nil {
		return nil, err
	}

	r := schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: toolchainEventResourcePlural}
	listOptions := v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", crtapi.ToolchainEventActivationCodeLabelKey, activationCode),
	}

	list, err := intf.Resource(r).Namespace(c.ns).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	result := &crtapi.ToolchainEventList{}

	err = c.crtClient.scheme.Convert(list, result, nil)
	return result, err
}
