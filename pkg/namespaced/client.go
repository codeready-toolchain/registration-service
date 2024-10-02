package namespaced

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewClient(client client.Client, namespace string) Client {
	return Client{Client: client, Namespace: namespace}
}

type Client struct {
	client.Client
	Namespace string
}

func (c Client) NamespacedName(name string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      name,
	}
}
