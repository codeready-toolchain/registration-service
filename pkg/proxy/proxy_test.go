package proxy

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/codeready-toolchain/registration-service/test"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestProxySuite struct {
	test.UnitTestSuite
}

func TestRunProxySuite(t *testing.T) {
	suite.Run(t, &TestProxySuite{test.UnitTestSuite{}})
}

func (s *TestProxySuite) TestProxy() {
	// given

	env := s.DefaultConfig().Environment()
	s.SetConfig(testconfig.RegistrationService().
		Environment(string(testconfig.E2E))) // We use e2e-test environment just to be able to re-use token generation
	defer s.SetConfig(testconfig.RegistrationService().
		Environment(env))

	_, err := auth.InitializeDefaultTokenParser()
	require.NoError(s.T(), err)
	fakeApp := &fakeApp{}
	p, err := newProxyWithClusterClient(fakeApp, configuration.GetRegistrationServiceConfig(), nil)
	require.NoError(s.T(), err)

	server := p.StartProxy()
	require.NotNil(s.T(), server)
	defer func() {
		_ = server.Close()
	}()

	// Wait up to 5 seconds for the Proxy server to start
	for i := 0; i < 5; i++ {
		log.Println("Checking if Proxy is started...")
		req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), req)
		_, err = http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		// Server is up and running!
		break
	}

	s.Run("return unauthorized if no token present", func() {
		req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), req)

		// when
		resp, err := http.DefaultClient.Do(req)

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), resp)
		assert.Equal(s.T(), http.StatusUnauthorized, resp.StatusCode)
		s.assertResponseBody(resp, "unable to create a context:no token found:a Bearer token is expected\n")
	})

	s.Run("unauthorized if can't parse token", func() {
		// when
		req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), req)
		req.Header.Set("Authorization", "Bearer not-a-token")
		resp, err := http.DefaultClient.Do(req)

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), resp)
		assert.Equal(s.T(), http.StatusUnauthorized, resp.StatusCode)
		s.assertResponseBody(resp, "unable to create a context:unable to extract userID from token:token contains an invalid number of segments\n")
	})

	s.Run("unauthorized if can't extract userID from a valid token", func() {
		// when
		req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), req)
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)
		req.Header.Set("Authorization", "Bearer "+s.token(userID, authsupport.WithSubClaim("")))
		resp, err := http.DefaultClient.Do(req)

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), resp)
		assert.Equal(s.T(), http.StatusUnauthorized, resp.StatusCode)
		s.assertResponseBody(resp, "unable to create a context:unable to extract userID from token:token does not comply to expected claims: subject missing\n")
	})

	s.Run("internal error if get namespace returns an error", func() {
		// given
		req, _ := s.request()
		fakeApp.namespaces = map[string]*namespace.Namespace{}
		fakeApp.err = errors.New("some-error")

		// when
		resp, err := http.DefaultClient.Do(req)

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), resp)
		assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
		s.assertResponseBody(resp, "unable to get target namespace:some-error\n")
	})

	s.Run("successfully Proxy", func() {
		// given
		req, userID := s.request()
		fakeApp.err = nil
		member1, err := url.Parse("https://member-1.openshift.com:1111")
		require.NoError(s.T(), err)

		// Start the member-2 API Server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("my response"))
			require.NoError(s.T(), err)
			assert.Equal(s.T(), "Bearer clusterSAToken", r.Header.Get("Authorization"))
		}))
		defer ts.Close()

		member2, err := url.Parse(ts.URL)
		require.NoError(s.T(), err)

		fakeApp.namespaces = map[string]*namespace.Namespace{
			"someUserID": { // noise
				APIURL:             *member1,
				TargetClusterToken: "",
			},
			userID: {
				APIURL:             *member2,
				TargetClusterToken: "clusterSAToken",
			},
		}

		// when
		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), resp)
		assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
		s.assertResponseBody(resp, "my response")
	})
}

func (s *TestProxySuite) TestSingleJoiningSlash() {
	assert.Equal(s.T(), "/", singleJoiningSlash("", ""))
	assert.Equal(s.T(), "/", singleJoiningSlash("/", "/"))
	assert.Equal(s.T(), "/api/namespace/pods", singleJoiningSlash("", "api/namespace/pods"))
	assert.Equal(s.T(), "proxy/", singleJoiningSlash("proxy", ""))
	assert.Equal(s.T(), "proxy/", singleJoiningSlash("proxy", "/"))
	assert.Equal(s.T(), "proxy/api/namespace/pods", singleJoiningSlash("proxy", "api/namespace/pods"))
	assert.Equal(s.T(), "proxy/subpath/api/namespace/pods", singleJoiningSlash("proxy/subpath", "api/namespace/pods"))
	assert.Equal(s.T(), "/proxy/subpath/api/namespace/pods/", singleJoiningSlash("/proxy/subpath/", "/api/namespace/pods/"))
}

func (s *TestProxySuite) request() (*http.Request, string) {
	req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), req)
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	req.Header.Set("Authorization", "Bearer "+s.token(userID))

	return req, userID.String()
}

func (s *TestProxySuite) token(userID uuid.UUID, extraClaims ...authsupport.ExtraClaim) string {
	userIdentity := &authsupport.Identity{
		ID:       userID,
		Username: "username-" + userID.String(),
	}

	extra := append(extraClaims, authsupport.WithEmailClaim("someemail@comp.com"))
	token, err := authsupport.GenerateSignedE2ETestToken(*userIdentity, extra...)
	require.NoError(s.T(), err)

	return token
}

func (s *TestProxySuite) assertResponseBody(resp *http.Response, expectedBody string) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(resp.Body)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), expectedBody, buf.String())
}

type fakeApp struct {
	namespaces map[string]*namespace.Namespace
	err        error
}

func (a *fakeApp) SignupService() service.SignupService {
	panic("SignupService shouldn't be called")
}

func (a *fakeApp) VerificationService() service.VerificationService {
	panic("VerificationService shouldn't be called")
}

func (a *fakeApp) MemberClusterService() service.MemberClusterService {
	return &fakeClusterService{a}
}

type fakeClusterService struct {
	fakeApp *fakeApp
}

func (f *fakeClusterService) GetNamespace(ctx *gin.Context, userID string) (*namespace.Namespace, error) {
	return f.fakeApp.namespaces[userID], f.fakeApp.err
}
