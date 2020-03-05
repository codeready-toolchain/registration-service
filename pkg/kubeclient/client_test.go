package kubeclient_test

import (
	"testing"

	"k8s.io/client-go/rest"

	"github.com/stretchr/testify/require"

	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
)

const (
	TestNamespace = "test-namespace-name"
)

func TestNewClient(t *testing.T) {
	// Try creating a new client with an empty config
	client, err := kubeclient.NewCRTV1Alpha1Client(&rest.Config{}, TestNamespace)

	// Check that there are no errors, and the clients are returned
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, client.UserSignups())
	require.NotNil(t, client.MasterUserRecords())
	require.NotNil(t, client.BannedUsers())
}
