package server_test

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/test/fake"

	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/registration-service/test"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestServerSuite struct {
	test.UnitTestSuite
}

func TestRunServerSuite(t *testing.T) {
	suite.Run(t, &TestServerSuite{test.UnitTestSuite{}})
}

func (s *TestServerSuite) TestServer() {
	// We're using the example config for the configuration here as the
	// specific config params do not matter for testing the routes setup.
	srv := server.New(s.Config, server.WithApplication(fake.NewMockableApplication(s.Config, nil)))

	// Setting up the routes.
	err := srv.SetupRoutes()
	require.NoError(s.T(), err)

	// Check that there are routes registered.
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(s.T(), routes)

	// Check that Engine() returns the router object.
	require.NotNil(s.T(), srv.Engine())
}
