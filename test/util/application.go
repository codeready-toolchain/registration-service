package util

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PrepareInClusterApp(t *testing.T, objects ...client.Object) (*commontest.FakeClient, application.Application) {
	return PrepareInClusterAppWithOption(t, func(_ *factory.ServiceFactory) {
	}, objects...)
}

func PrepareInClusterAppWithOption(t *testing.T, option factory.Option, objects ...client.Object) (*commontest.FakeClient, application.Application) {
	fakeClient := commontest.NewFakeClient(t, objects...)
	app := server.NewInClusterApplication(fakeClient, commontest.HostOperatorNs, option)
	return fakeClient, app
}
