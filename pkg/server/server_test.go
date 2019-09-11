package registrationserver_test

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	// We're using the example config for the configuration here as the
	// specific config params do not matter for testing the routes setup.
	srv, err := registrationserver.New("../../example-config.yml")
	require.NoError(t, err)

	// Setting up the routes.
	err = srv.SetupRoutes()
	require.NoError(t, err)

	// Check that there are routes registered.
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(t, routes)

	// Check that Engine() returns the router object.
	require.NotNil(t, srv.Engine())
}
