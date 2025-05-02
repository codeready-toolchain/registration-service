package util

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PrepareInClusterApplication(t *testing.T, objects ...client.Object) application.Application {
	_, app := PrepareInClusterApp(t, objects...)
	return app
}

func PrepareInClusterApp(t *testing.T, objects ...client.Object) (*commontest.FakeClient, application.Application) {
	fakeClient := commontest.NewFakeClient(t, objects...)
	app := server.NewInClusterApplication(namespaced.NewClient(fakeClient, commontest.HostOperatorNs))
	return fakeClient, app
}
