package proxy

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
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
	defer s.SetConfig(testconfig.RegistrationService().
		Environment(env))
	s.SetConfig(testconfig.RegistrationService().
		Environment(string(testconfig.E2E))) // We use e2e-test environment just to be able to re-use token generation
	_, err := auth.InitializeDefaultTokenParser()
	require.NoError(s.T(), err)

	for _, environment := range []testconfig.EnvName{testconfig.E2E, testconfig.Dev, testconfig.Prod} {
		s.Run("for environment "+string(environment), func() {

			s.SetConfig(testconfig.RegistrationService().
				Environment(string(environment)))
			fakeApp := &fakeApp{}
			p, err := newProxyWithClusterClient(fakeApp, nil)
			require.NoError(s.T(), err)

			server := p.StartProxy()
			require.NotNil(s.T(), server)
			defer func() {
				_ = server.Close()
			}()

			// Wait up to N seconds for the Proxy server to start
			ready := false
			sec := 10
			for i := 0; i < sec; i++ {
				log.Println("Checking if Proxy is started...")
				req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
				require.NoError(s.T(), err)
				require.NotNil(s.T(), req)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					time.Sleep(time.Second)
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusUnauthorized {
					// The server may be running but still not fully ready to accept requests
					time.Sleep(time.Second)
					continue
				}
				// Server is up and running!
				ready = true
				break
			}
			require.True(s.T(), ready, "Proxy is not ready after %d seconds", sec)

			s.Run("health check ok", func() {
				req, err := http.NewRequest("GET", "http://localhost:8081/proxyhealth", nil)
				require.NoError(s.T(), err)
				require.NotNil(s.T(), req)

				// when
				resp, err := http.DefaultClient.Do(req)

				// then
				require.NoError(s.T(), err)
				require.NotNil(s.T(), resp)
				assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
				s.assertResponseBody(resp, `{"alive": true}`)
			})

			s.Run("plain http error", func() {
				s.Run("unauthorized if no token present", func() {
					req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
					require.NoError(s.T(), err)
					require.NotNil(s.T(), req)

					// when
					resp, err := http.DefaultClient.Do(req)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), resp)
					assert.Equal(s.T(), http.StatusUnauthorized, resp.StatusCode)
					s.assertResponseBody(resp, "invalid bearer token:no token found:a Bearer token is expected\n")
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
					s.assertResponseBody(resp, "invalid bearer token:unable to extract userID from token:token contains an invalid number of segments\n")
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
					s.assertResponseBody(resp, "invalid bearer token:unable to extract userID from token:token does not comply to expected claims: subject missing\n")
				})

				s.Run("internal error if get namespace returns an error", func() {
					// given
					req, _ := s.request()
					fakeApp.namespaces = map[string]*namespace.NamespaceAccess{}
					fakeApp.err = errors.New("some-error")

					// when
					resp, err := http.DefaultClient.Do(req)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), resp)
					assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
					s.assertResponseBody(resp, "unable to get target namespace:some-error\n")
				})
			})

			s.Run("websockets error", func() {
				tests := map[string]struct {
					ProtocolHeaders []string
					ExpectedError   string
				}{
					"empty token": {
						ProtocolHeaders: []string{"base64url.bearer.authorization.k8s.io.,dummy"},
						ExpectedError:   "invalid bearer token:no base64.bearer.authorization token found",
					},
					"not a jwt token": {
						ProtocolHeaders: []string{"base64url.bearer.authorization.k8s.io.dG9rZW4,dummy"},
						ExpectedError:   "invalid bearer token:unable to extract userID from token:token contains an invalid number of segments",
					},
					"invalid token is not base64 encoded": {
						ProtocolHeaders: []string{"base64url.bearer.authorization.k8s.io.token,dummy"},
						ExpectedError:   "invalid bearer token:invalid base64.bearer.authorization token encoding: illegal base64 data at input byte 4",
					},
					"invalid token contains non UTF-8-encoded runes": {
						ProtocolHeaders: []string{fmt.Sprintf("base64url.bearer.authorization.k8s.io.%s,dummy", base64.RawURLEncoding.EncodeToString([]byte("aa\xe2")))},
						ExpectedError:   "invalid bearer token:invalid base64.bearer.authorization token: contains non UTF-8-encoded runes",
					},
					"no header": {
						ProtocolHeaders: nil,
						ExpectedError:   "invalid bearer token:no base64.bearer.authorization token found",
					},
					"empty header": {
						ProtocolHeaders: []string{""},
						ExpectedError:   "invalid bearer token:no base64.bearer.authorization token found",
					},
					"non-bearer header": {
						ProtocolHeaders: []string{"undefined"},
						ExpectedError:   "invalid bearer token:no base64.bearer.authorization token found",
					},
					"empty bearer token": {
						ProtocolHeaders: []string{"base64url.bearer.authorization.k8s.io."},
						ExpectedError:   "invalid bearer token:no base64.bearer.authorization token found",
					},
					"multiple bearer tokens": {
						ProtocolHeaders: []string{
							"base64url.bearer.authorization.k8s.io.dG9rZW4,dummy",
							"base64url.bearer.authorization.k8s.io.dG9rZW4,dummy",
						},
						ExpectedError: "invalid bearer token:multiple base64.bearer.authorization tokens specified",
					},
				}

				for k, tc := range tests {
					s.Run(k, func() {
						req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
						require.NoError(s.T(), err)
						require.NotNil(s.T(), req)
						upgradeToWebsocket(req)
						for _, h := range tc.ProtocolHeaders {
							req.Header.Add("Sec-Websocket-Protocol", h)
						}

						// when
						resp, err := http.DefaultClient.Do(req)

						// then
						require.NoError(s.T(), err)
						require.NotNil(s.T(), resp)
						assert.Equal(s.T(), http.StatusUnauthorized, resp.StatusCode)
						s.assertResponseBody(resp, tc.ExpectedError+"\n")
					})
				}
			})

			s.Run("successfully proxy", func() {
				userID, err := uuid.NewV4()
				require.NoError(s.T(), err)

				encodedSAToken := base64.RawURLEncoding.EncodeToString([]byte("clusterSAToken"))
				encodedSSOToken := base64.RawURLEncoding.EncodeToString([]byte(s.token(userID)))

				tests := map[string]struct {
					ProxyHeaders                    map[string]string
					ExpectedAPIServerRequestHeaders map[string]string
					ExpectedProxyResponseHeaders    map[string]string
				}{
					"plain http": {
						ProxyHeaders:                    map[string]string{"Authorization": "Bearer " + s.token(userID)},
						ExpectedAPIServerRequestHeaders: map[string]string{"Authorization": "Bearer clusterSAToken"},
						ExpectedProxyResponseHeaders: map[string]string{
							"Access-Control-Allow-Origin":      "*",
							"Access-Control-Allow-Credentials": "true",
							"Access-Control-Expose-Headers":    "Content-Encoding,Authorization",
						},
					},
					"websockets": {
						ProxyHeaders: map[string]string{
							"Connection":             "upgrade",
							"Upgrade":                "websocket",
							"Sec-Websocket-Protocol": fmt.Sprintf("base64url.bearer.authorization.k8s.io.%s,dummy", encodedSSOToken),
						},
						ExpectedAPIServerRequestHeaders: map[string]string{
							"Connection":             "Upgrade",
							"Upgrade":                "websocket",
							"Sec-Websocket-Protocol": fmt.Sprintf("base64url.bearer.authorization.k8s.io.%s,dummy", encodedSAToken),
						},
						ExpectedProxyResponseHeaders: map[string]string{
							"Access-Control-Allow-Origin":      "*",
							"Access-Control-Allow-Credentials": "true",
							"Access-Control-Expose-Headers":    "Content-Encoding,Authorization",
						},
					},
				}

				for k, tc := range tests {
					s.Run(k, func() {
						// given
						req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
						require.NoError(s.T(), err)
						require.NotNil(s.T(), req)

						for hk, hv := range tc.ProxyHeaders {
							req.Header.Set(hk, hv)
						}

						fakeApp.err = nil
						member1, err := url.Parse("https://member-1.openshift.com:1111")
						require.NoError(s.T(), err)

						// Start the member-2 API Server
						ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusOK)
							_, err := w.Write([]byte("my response"))
							require.NoError(s.T(), err)
							for hk, hv := range tc.ExpectedAPIServerRequestHeaders {
								assert.Equal(s.T(), hv, r.Header.Get(hk))
							}
						}))
						defer ts.Close()

						member2, err := url.Parse(ts.URL)
						require.NoError(s.T(), err)

						fakeApp.namespaces = map[string]*namespace.NamespaceAccess{
							"someUserID": { // noise
								APIURL:  *member1,
								SAToken: "",
							},
							userID.String(): {
								APIURL:  *member2,
								SAToken: "clusterSAToken",
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
						for hk, hv := range tc.ExpectedProxyResponseHeaders {
							assert.Equal(s.T(), hv, resp.Header.Get(hk))
						}
					})
				}
			})
		})
	}
}

func upgradeToWebsocket(req *http.Request) {
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
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
	namespaces map[string]*namespace.NamespaceAccess
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

func (f *fakeClusterService) GetNamespace(_ *gin.Context, userID string) (*namespace.NamespaceAccess, error) {
	return f.fakeApp.namespaces[userID], f.fakeApp.err
}
