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
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"
	"k8s.io/client-go/rest"

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

			server := p.StartProxy(&rest.Config{})
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
					s.assertResponseBody(resp, "invalid bearer token: no token found: a Bearer token is expected\n")
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
					s.assertResponseBody(resp, "invalid bearer token: unable to extract userID from token: token contains an invalid number of segments\n")
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
					s.assertResponseBody(resp, "invalid bearer token: unable to extract userID from token: token does not comply to expected claims: subject missing\n")
				})

				s.Run("internal error if get accesses returns an error", func() {
					// given
					req, _ := s.request()
					fakeApp.accesses = map[string]*access.ClusterAccess{}
					fakeApp.err = errors.New("some-error")

					// when
					resp, err := http.DefaultClient.Do(req)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), resp)
					assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
					s.assertResponseBody(resp, "unable to get target cluster: some-error\n")
				})
			})

			s.Run("websockets error", func() {
				tests := map[string]struct {
					ProtocolHeaders []string
					ExpectedError   string
				}{
					"empty token": {
						ProtocolHeaders: []string{"base64url.bearer.authorization.k8s.io.,dummy"},
						ExpectedError:   "invalid bearer token: no base64.bearer.authorization token found",
					},
					"not a jwt token": {
						ProtocolHeaders: []string{"base64url.bearer.authorization.k8s.io.dG9rZW4,dummy"},
						ExpectedError:   "invalid bearer token: unable to extract userID from token: token contains an invalid number of segments",
					},
					"invalid token is not base64 encoded": {
						ProtocolHeaders: []string{"base64url.bearer.authorization.k8s.io.token,dummy"},
						ExpectedError:   "invalid bearer token: invalid base64.bearer.authorization token encoding: illegal base64 data at input byte 4",
					},
					"invalid token contains non UTF-8-encoded runes": {
						ProtocolHeaders: []string{fmt.Sprintf("base64url.bearer.authorization.k8s.io.%s,dummy", base64.RawURLEncoding.EncodeToString([]byte("aa\xe2")))},
						ExpectedError:   "invalid bearer token: invalid base64.bearer.authorization token: contains non UTF-8-encoded runes",
					},
					"no header": {
						ProtocolHeaders: nil,
						ExpectedError:   "invalid bearer token: no base64.bearer.authorization token found",
					},
					"empty header": {
						ProtocolHeaders: []string{""},
						ExpectedError:   "invalid bearer token: no base64.bearer.authorization token found",
					},
					"non-bearer header": {
						ProtocolHeaders: []string{"undefined"},
						ExpectedError:   "invalid bearer token: no base64.bearer.authorization token found",
					},
					"empty bearer token": {
						ProtocolHeaders: []string{"base64url.bearer.authorization.k8s.io."},
						ExpectedError:   "invalid bearer token: no base64.bearer.authorization token found",
					},
					"multiple bearer tokens": {
						ProtocolHeaders: []string{
							"base64url.bearer.authorization.k8s.io.dG9rZW4,dummy",
							"base64url.bearer.authorization.k8s.io.dG9rZW4,dummy",
						},
						ExpectedError: "invalid bearer token: multiple base64.bearer.authorization tokens specified",
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

				// Start the member-2 API Server
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					// Set the Access-Control-Allow-Origin header to make sure it's overridden by the proxy response modifier
					w.Header().Set("Access-Control-Allow-Origin", "dummy")
					w.WriteHeader(http.StatusOK)
					_, err := w.Write([]byte("my response"))
					require.NoError(s.T(), err)
				}))
				defer testServer.Close()
				member2, err := url.Parse(testServer.URL)
				require.NoError(s.T(), err)

				tests := map[string]struct {
					ProxyRequestMethod              string
					ProxyRequestHeaders             http.Header
					ExpectedAPIServerRequestHeaders http.Header
					ExpectedProxyResponseHeaders    http.Header
					ExpectedProxyResponseStatus     int
					Standalone                      bool // If true then the request is not expected to be forwarded to the kube api server
				}{
					"plain http cors preflight request with no request method": {
						ProxyRequestMethod: "OPTIONS",
						ProxyRequestHeaders: map[string][]string{
							"Origin": {"https://domain.com"},
						},
						ExpectedProxyResponseHeaders: noCORSHeaders,
						ExpectedProxyResponseStatus:  http.StatusUnauthorized,
						Standalone:                   true,
					},
					"plain http cors preflight request with unknown request method": {
						ProxyRequestMethod: "OPTIONS",
						ProxyRequestHeaders: map[string][]string{
							"Origin":                        {"https://domain.com"},
							"Access-Control-Request-Method": {"UNKNOWN"},
						},
						ExpectedProxyResponseHeaders: noCORSHeaders,
						ExpectedProxyResponseStatus:  http.StatusNoContent,
						Standalone:                   true,
					},
					"plain http cors preflight request with no origin": {
						ProxyRequestMethod: "OPTIONS",
						ProxyRequestHeaders: map[string][]string{
							"Access-Control-Request-Method": {"GET"},
						},
						ExpectedProxyResponseHeaders: noCORSHeaders,
						ExpectedProxyResponseStatus:  http.StatusNoContent,
						Standalone:                   true,
					},
					"plain http cors preflight request": {
						ProxyRequestMethod: "OPTIONS",
						ProxyRequestHeaders: map[string][]string{
							"Origin":                         {"https://domain.com"},
							"Access-Control-Request-Method":  {"GET"},
							"Access-Control-Request-Headers": {"Authorization"},
						},
						ExpectedProxyResponseHeaders: map[string][]string{
							"Access-Control-Allow-Origin":      {"https://domain.com"},
							"Access-Control-Allow-Credentials": {"true"},
							"Access-Control-Allow-Headers":     {"Authorization"},
							"Access-Control-Allow-Methods":     {"PUT, PATCH, POST, GET, DELETE, OPTIONS"},
							"Vary":                             {"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"},
						},
						ExpectedProxyResponseStatus: http.StatusNoContent,
						Standalone:                  true,
					},
					"plain http cors preflight request multiple request headers": {
						ProxyRequestMethod: "OPTIONS",
						ProxyRequestHeaders: map[string][]string{
							"Origin":                         {"https://domain.com"},
							"Access-Control-Request-Method":  {"GET"},
							"Access-Control-Request-Headers": {"Authorization, content-Type, header, second-header, THIRD-HEADER, Numb3r3d-H34d3r"},
						},
						ExpectedProxyResponseHeaders: map[string][]string{
							"Access-Control-Allow-Origin":      {"https://domain.com"},
							"Access-Control-Allow-Credentials": {"true"},
							"Access-Control-Allow-Headers":     {"Authorization, Content-Type, Header, Second-Header, Third-Header, Numb3r3d-H34d3r"},
							"Access-Control-Allow-Methods":     {"PUT, PATCH, POST, GET, DELETE, OPTIONS"},
							"Vary":                             {"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"},
						},
						ExpectedProxyResponseStatus: http.StatusNoContent,
						Standalone:                  true,
					},
					"plain http actual request": {
						ProxyRequestMethod:              "GET",
						ProxyRequestHeaders:             map[string][]string{"Authorization": {"Bearer " + s.token(userID)}},
						ExpectedAPIServerRequestHeaders: map[string][]string{"Authorization": {"Bearer clusterSAToken"}},
						ExpectedProxyResponseHeaders: map[string][]string{
							"Access-Control-Allow-Origin":      {"*"},
							"Access-Control-Allow-Credentials": {"true"},
							"Access-Control-Expose-Headers":    {"Content-Length, Content-Encoding, Authorization"},
							"Vary":                             {"Origin"},
						},
						ExpectedProxyResponseStatus: http.StatusOK,
					},
					"websockets": {
						ProxyRequestMethod: "GET",
						ProxyRequestHeaders: map[string][]string{
							"Connection":             {"upgrade"},
							"Upgrade":                {"websocket"},
							"Sec-Websocket-Protocol": {fmt.Sprintf("base64url.bearer.authorization.k8s.io.%s,dummy", encodedSSOToken)},
						},
						ExpectedAPIServerRequestHeaders: map[string][]string{
							"Connection":             {"Upgrade"},
							"Upgrade":                {"websocket"},
							"Sec-Websocket-Protocol": {fmt.Sprintf("base64url.bearer.authorization.k8s.io.%s,dummy", encodedSAToken)},
						},
						ExpectedProxyResponseHeaders: map[string][]string{
							"Access-Control-Allow-Origin":      {"*"},
							"Access-Control-Allow-Credentials": {"true"},
							"Access-Control-Expose-Headers":    {"Content-Length, Content-Encoding, Authorization"},
							"Vary":                             {"Origin"},
						},
						ExpectedProxyResponseStatus: http.StatusOK,
					},
				}

				for k, tc := range tests {
					s.Run(k, func() {
						// given
						req, err := http.NewRequest(tc.ProxyRequestMethod, "http://localhost:8081/api/mycoolworkspace/pods", nil)
						require.NoError(s.T(), err)
						require.NotNil(s.T(), req)

						for hk, hv := range tc.ProxyRequestHeaders {
							for _, v := range hv {
								req.Header.Add(hk, v)
							}
						}

						fakeApp.err = nil
						member1, err := url.Parse("https://member-1.openshift.com:1111")
						require.NoError(s.T(), err)

						if !tc.Standalone {
							testServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								w.Header().Set("Content-Type", "application/json")
								// Set the Access-Control-Allow-Origin header to make sure it's overridden by the proxy response modifier
								w.Header().Set("Access-Control-Allow-Origin", "dummy")
								w.WriteHeader(http.StatusOK)
								_, err := w.Write([]byte("my response"))
								require.NoError(s.T(), err)
								for hk, hv := range tc.ExpectedAPIServerRequestHeaders {
									require.Len(s.T(), r.Header.Values(hk), len(hv))
									for i := range hv {
										assert.Equal(s.T(), hv[i], r.Header.Values(hk)[i])
									}
								}
							})

							fakeApp.accesses = map[string]*access.ClusterAccess{
								"someUserID":    access.NewClusterAccess(*member1, "", ""), // noise
								userID.String(): access.NewClusterAccess(*member2, "clusterSAToken", ""),
							}
						}

						// when
						client := http.Client{Timeout: 3 * time.Second}
						resp, err := client.Do(req)

						// then
						require.NoError(s.T(), err)
						require.NotNil(s.T(), resp)
						assert.Equal(s.T(), tc.ExpectedProxyResponseStatus, resp.StatusCode)
						if !tc.Standalone {
							s.assertResponseBody(resp, "my response")
						}
						for hk, hv := range tc.ExpectedProxyResponseHeaders {
							require.Len(s.T(), resp.Header.Values(hk), len(hv), fmt.Sprintf("Actual Header %s: %v", hk, resp.Header.Values(hk)))
							for i := range hv {
								assert.Equal(s.T(), hv[i], resp.Header.Values(hk)[i])
							}
						}
					})
				}
			})
		})
	}
}

var noCORSHeaders = map[string][]string{
	"Access-Control-Allow-Origin":      {},
	"Access-Control-Allow-Credentials": {},
	"Access-Control-Allow-Headers":     {},
	"Access-Control-Allow-Methods":     {},
	"Vary":                             {},
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
	accesses map[string]*access.ClusterAccess
	err      error
}

func (a *fakeApp) InformerService() service.InformerService {
	panic("InformerService shouldn't be called")
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

func (f *fakeClusterService) GetClusterAccess(userID, _ string) (*access.ClusterAccess, error) {
	return f.fakeApp.accesses[userID], f.fakeApp.err
}

func (f *fakeClusterService) GetSignupViaInformers(userID, username string) (*signup.Signup, error) {
	panic("GetSignupViaInformers shouldn't be called")
}
