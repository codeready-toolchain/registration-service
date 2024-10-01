package util

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/registration-service/test/fake"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewMemberClusterServiceContext(t *testing.T, cl client.Client) fake.MemberClusterServiceContext {
	crtClient, err := kubeclient.NewCRTRESTClient(cl, commontest.HostOperatorNs)
	require.NoError(t, err)
	application, err := server.NewInClusterApplication(cl, commontest.HostOperatorNs)
	require.NoError(t, err)
	return fake.MemberClusterServiceContext{
		CrtClient: crtClient,
		Svcs:      application,
	}
}
