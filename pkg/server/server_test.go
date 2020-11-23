package server_test

import (
	"github.com/codeready-toolchain/registration-service/test/fake"
	"k8s.io/apimachinery/pkg/util/wait"
	"net/http"
	"testing"
	"time"

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

const (
	DefaultRetryInterval = time.Millisecond * 100 // make it short because a "retry interval" is waited before the first test
	DefaultTimeout       = time.Second * 30
)

func (s *TestServerSuite) TestServer() {
	// We're using the example config for the configuration here as the
	// specific config params do not matter for testing the routes setup.
	srv := server.New(s.Config(), fake.NewMockableApplication(s.Config(), nil))

	// Setting up the routes.
	err := srv.SetupRoutes()
	require.NoError(s.T(), err)

	// Check that there are routes registered.
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(s.T(), routes)

	// Check that Engine() returns the router object.
	require.NotNil(s.T(), srv.Engine())

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	s.T().Run("CORS", func(t *testing.T) {
		go srv.Engine().Run()

		err := wait.Poll(DefaultRetryInterval, DefaultTimeout, func() (done bool, err error) {
			req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/health", nil)
			if err != nil {
				return false, err
			}

			resp, err := client.Do(req)
			if err != nil {
				// We will ignore and try again until we don't get any error or timeout.
				return false, nil
			}

			if resp.StatusCode != 200 {
				return false, nil
			}

			return true, nil
		})
		require.NoError(s.T(), err)

		req, err := http.NewRequest("OPTIONS", "http://localhost:8080/api/v1/authconfig", nil)
		require.NoError(s.T(), err)

		req.Header.Set("Origin", "http://example.com")

		resp, err := client.Do(req)
		require.NoError(s.T(), err)

		defer resp.Body.Close()

		require.Equal(s.T(), 204, resp.StatusCode)
		require.Equal(s.T(), "Content-Length,Content-Type,Authorization,Accept", resp.Header.Get("Access-Control-Allow-Headers"))
		require.Equal(s.T(), "PUT,PATCH,POST,GET,DELETE,OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))
		require.Equal(s.T(), "*", resp.Header.Get("Access-Control-Allow-Origin"))
		require.Equal(s.T(), "true", resp.Header.Get("Access-Control-Allow-Credentials"))
	})
}
