package kubeclient

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestNewKubeClient(t *testing.T) {
	k := KubeClient{}
	k.CoreClient = fake.NewSimpleClientset()
	assert.NotNil(t, k.CoreClient, "Kubecore client shouldn't be nil")
}
