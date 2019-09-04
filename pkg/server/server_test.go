package registrationserver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codeready-toolchain/registration-service/pkg/server"
)

func TestServer(t *testing.T) {
	// we're using the example config for the configuration here as the
	// specific config params do not matter for testing the routes setup.
	srv, err := registrationserver.New("../../example-config.yml")
	require.NoError(t, err)

	// setting up the routes.
	err = srv.SetupRoutes()
	require.NoError(t, err)

	// check that there are routes registered
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(t, routes)
}
