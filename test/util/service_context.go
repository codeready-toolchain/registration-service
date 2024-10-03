package util

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/registration-service/test/fake"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewMemberClusterServiceContext(_ *testing.T, cl client.Client) fake.MemberClusterServiceContext {
	return fake.MemberClusterServiceContext{
		NamespacedClient: namespaced.NewClient(cl, commontest.HostOperatorNs),
		Svcs:             server.NewInClusterApplication(cl, commontest.HostOperatorNs),
	}
}
