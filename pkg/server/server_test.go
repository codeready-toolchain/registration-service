package server_test

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/test/fake"
	"gopkg.in/h2non/gock.v1"
	"k8s.io/apimachinery/pkg/util/wait"

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
	srv := server.New(fake.NewMockableApplication(nil))

	fake.MockKeycloakCertsCall(s.T())
	// Setting up the routes.
	err := srv.SetupRoutes("8091")
	require.NoError(s.T(), err)
	gock.OffAll()

	startFakeProxy(s.T())

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
		go func(t *testing.T) {
			err := srv.Engine().Run()
			require.NoError(t, err)
		}(t)

		err := wait.Poll(DefaultRetryInterval, DefaultTimeout, func() (done bool, err error) {
			req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/health", nil)
			if err != nil {
				return false, err
			}

			resp, err := client.Do(req)
			defer func() {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}()
			if err != nil {
				// We will ignore and try again until we don't get any error or timeout.
				return false, nil // nolint:nilerr
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
		require.Equal(s.T(), "Content-Length,Content-Type,Authorization,Accept,Recaptcha-Token", resp.Header.Get("Access-Control-Allow-Headers"))
		require.Equal(s.T(), "PUT,PATCH,POST,GET,DELETE,OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))
		require.Equal(s.T(), "*", resp.Header.Get("Access-Control-Allow-Origin"))
		require.Equal(s.T(), "true", resp.Header.Get("Access-Control-Allow-Credentials"))
	})
}

func startFakeProxy(t *testing.T) *http.Server {
	// start server
	mux := http.NewServeMux()
	mux.HandleFunc("/proxyhealth", fakehealth)

	// use an unique port for proxy to avoid collisions with tests in `pkg/proxy`
	// that are using the pkg/proxy's default port (i.e. proxy.ProxyPort 8081)
	altProxyPort := "8091"
	srv := &http.Server{Addr: ":" + altProxyPort, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	go func() {
		err := srv.ListenAndServe()
		require.NoError(t, err)
	}()
	return srv
}

func fakehealth(res http.ResponseWriter, _ *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	io.WriteString(res, `{"alive": true}`) //nolint:golint,errcheck
}
