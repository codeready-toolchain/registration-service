package server_test

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestServerSuite struct {
	testutils.UnitTestSuite
}

func TestRunServerSuite(t *testing.T) {
	suite.Run(t, &TestServerSuite{testutils.UnitTestSuite{}})
}

func (s *TestServerSuite) TestServer() {
	// We're using the example config for the configuration here as the
	// specific config params do not matter for testing the routes setup.
	srv, err := server.New("../../example-config.yml")
	require.NoError(s.T(), err)

	// Setting up the routes.
	err = srv.SetupRoutes()
	require.NoError(s.T(), err)

	// Check that there are routes registered.
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(s.T(), routes)

	// Check that Engine() returns the router object.
	require.NotNil(s.T(), srv.Engine())
}
