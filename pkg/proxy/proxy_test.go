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

	appservice "github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/service"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	commoncluster "github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
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
			fakeApp := &fake.ProxyFakeApp{}
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
					s.assertResponseBody(resp, "invalid bearer token: no token found: a Bearer token is expected")
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
					s.assertResponseBody(resp, "invalid bearer token: unable to extract userID from token: token contains an invalid number of segments")
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
					s.assertResponseBody(resp, "invalid bearer token: unable to extract userID from token: token does not comply to expected claims: subject missing")
				})

				s.Run("unauthorized if workspace context is invalid", func() {
					// when
					req, _ := s.request()
					req.URL.Path = "http://localhost:8081/workspaces/myworkspace" // invalid workspace context
					require.NotNil(s.T(), req)

					// when
					resp, err := http.DefaultClient.Do(req)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), resp)
					assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
					s.assertResponseBody(resp, "unable to get workspace context: workspace request path has too few segments '/workspaces/myworkspace'; expected path format: /workspaces/<workspace_name>/api/...")
				})

				s.Run("internal error if get accesses returns an error", func() {
					// given
					req, _ := s.request()
					fakeApp.Accesses = map[string]*access.ClusterAccess{}
					fakeApp.Err = errors.New("some-error")

					// when
					resp, err := http.DefaultClient.Do(req)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), resp)
					assert.Equal(s.T(), http.StatusInternalServerError, resp.StatusCode)
					s.assertResponseBody(resp, "unable to get target cluster: some-error")
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
						s.assertResponseBody(resp, tc.ExpectedError)
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
							"Origin":           {"https://domain.com"},
							"Authorization":    {"Bearer clusterSAToken"},
							"Impersonate-User": {"smith2"},
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
							"Authorization":                 {"Bearer clusterSAToken"},
							"Impersonate-User":              {"smith2"},
						},
						ExpectedProxyResponseHeaders: noCORSHeaders,
						ExpectedProxyResponseStatus:  http.StatusNoContent,
						Standalone:                   true,
					},
					"plain http cors preflight request with no origin": {
						ProxyRequestMethod: "OPTIONS",
						ProxyRequestHeaders: map[string][]string{
							"Access-Control-Request-Method": {"GET"},
							"Authorization":                 {"Bearer clusterSAToken"},
							"Impersonate-User":              {"smith2"},
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
							"Authorization":                  {"Bearer clusterSAToken"},
							"Impersonate-User":               {"smith2"},
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
							"Authorization":                  {"Bearer clusterSAToken"},
							"Impersonate-User":               {"smith2"},
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
						ProxyRequestMethod:  "GET",
						ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(userID)}},
						ExpectedAPIServerRequestHeaders: map[string][]string{
							"Authorization":    {"Bearer clusterSAToken"},
							"Impersonate-User": {"smith2"},
						},
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
							"Impersonate-User":       {"smith2"},
						},
						ExpectedAPIServerRequestHeaders: map[string][]string{
							"Connection":             {"Upgrade"},
							"Upgrade":                {"websocket"},
							"Sec-Websocket-Protocol": {fmt.Sprintf("base64url.bearer.authorization.k8s.io.%s,dummy", encodedSAToken)},
							"Impersonate-User":       {"smith2"},
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

						// Test each request using both the default workspace URL and a URL that uses the
						// workspace context. Both should yield the same results.
						for workspaceContext, reqPath := range map[string]string{
							"default workspace": "http://localhost:8081/api/mycoolworkspace/pods",
							"workspace context": "http://localhost:8081/workspaces/mycoolworkspace/api/mycoolworkspace/pods",
						} {
							s.Run(workspaceContext, func() {
								// given
								req, err := http.NewRequest(tc.ProxyRequestMethod, reqPath, nil)
								require.NoError(s.T(), err)
								require.NotNil(s.T(), req)

								for hk, hv := range tc.ProxyRequestHeaders {
									for _, v := range hv {
										req.Header.Add(hk, v)
									}
								}

								fakeApp.Err = nil

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
									fakeApp.SignupServiceMock = fake.NewSignupService(
										fake.Signup("someUserID", &signup.Signup{
											Name:              "smith1",
											APIEndpoint:       "https://api.endpoint.member-1.com:6443",
											ClusterName:       "member-1",
											CompliantUsername: "smith1",
											Username:          "smith1@",
											Status: signup.Status{
												Ready: true,
											},
										}),
										fake.Signup(userID.String(), &signup.Signup{
											Name:              "smith2",
											APIEndpoint:       testServer.URL,
											ClusterName:       "member-2",
											CompliantUsername: "smith2",
											Username:          "smith2@",
											Status: signup.Status{
												Ready: true,
											},
										}),
									)
									s.Application.MockSignupService(fakeApp.SignupServiceMock)
									inf := fake.NewFakeInformer()
									inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
										switch name {
										case "mycoolworkspace":
											return fake.NewSpace("mycoolworkspace", "member-2", "smith2"), nil
										}
										return nil, fmt.Errorf("space not found error")
									}
									inf.ListSpaceBindingFunc = func(reqs ...labels.Requirement) ([]*toolchainv1alpha1.SpaceBinding, error) {
										// always return a spacebinding for the purposes of the proxy tests, actual testing of the space lister is covered in the space lister tests
										spaceBindings := []*toolchainv1alpha1.SpaceBinding{}
										for _, req := range reqs {
											if req.Values().List()[0] == "smith2" {
												spaceBindings = append(spaceBindings, fake.NewSpaceBinding("mycoolworkspace-smith2", "smith2", "mycoolworkspace", "admin"))
											}
										}
										return spaceBindings, nil
									}
									s.Application.MockInformerService(inf)
									fakeApp.MemberClusterServiceMock = s.newMemberClusterServiceWithMembers(testServer.URL)

									p.spaceLister = &handlers.SpaceLister{
										GetSignupFunc: fakeApp.SignupServiceMock.GetSignupFromInformer,
										GetInformerServiceFunc: func() appservice.InformerService {
											return inf
										},
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
				}
			})

		})
	}
}

func (s *TestProxySuite) newMemberClusterServiceWithMembers(serverURL string) appservice.MemberClusterService {
	return service.NewMemberClusterService(
		fake.MemberClusterServiceContext{
			Client: s,
			Svcs:   s.Application,
		},
		func(si *service.ServiceImpl) {
			si.GetMembersFunc = func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
				return []*commoncluster.CachedToolchainCluster{
					{
						Config: &commoncluster.Config{
							Name:        "member-1",
							Type:        commoncluster.Member,
							APIEndpoint: "https://api.endpoint.member-1.com:6443",
							RestConfig:  &rest.Config{},
						},
					},
					{
						Config: &commoncluster.Config{
							Name:              "member-2",
							APIEndpoint:       serverURL,
							Type:              commoncluster.Member,
							OperatorNamespace: "member-operator",
							RestConfig: &rest.Config{
								BearerToken: "clusterSAToken",
							},
						},
						Client: commontest.NewFakeClient(s.T()),
					},
				}
			}
		},
	)
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

func (s *TestProxySuite) TestGetWorkspaceContext() {
	tests := map[string]struct {
		path              string
		expectedWorkspace string
		expectedPath      string
		expectedErr       string
	}{
		"valid workspace context": {
			path:              "/workspaces/myworkspace/api",
			expectedWorkspace: "myworkspace",
			expectedPath:      "/api",
			expectedErr:       "",
		},
		"invalid workspace context": {
			path:              "/workspaces/myworkspace",
			expectedWorkspace: "",
			expectedPath:      "/workspaces/myworkspace",
			expectedErr:       "workspace request path has too few segments '/workspaces/myworkspace'; expected path format: /workspaces/<workspace_name>/api/...",
		},
		"no workspace context": {
			path:              "/api/pods",
			expectedWorkspace: "",
			expectedPath:      "/api/pods",
			expectedErr:       "",
		},
		"workspace instead of workspaces": {
			path:              "/workspace/myworkspace/api",
			expectedWorkspace: "",
			expectedPath:      "/workspace/myworkspace/api",
			expectedErr:       "",
		},
	}

	for k, tc := range tests {
		s.T().Run(k, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					Path: tc.path,
				},
			}
			workspace, err := getWorkspaceContext(req)
			if tc.expectedErr == "" {
				require.NoError(s.T(), err)
			} else {
				require.EqualError(s.T(), err, tc.expectedErr)
			}
			assert.Equal(s.T(), tc.expectedWorkspace, workspace)
			assert.Equal(s.T(), tc.expectedPath, req.URL.Path)
		})
	}
}

func (s *TestProxySuite) TestValidateWorkspaceRequest() {
	tests := map[string]struct {
		requestedWorkspace string
		requestedNamespace string
		workspaces         []toolchainv1alpha1.Workspace
		expectedErr        string
	}{
		"valid workspace request": {
			requestedWorkspace: "myworkspace",
			requestedNamespace: "ns-dev",
			workspaces: []toolchainv1alpha1.Workspace{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myworkspace",
				},
				Status: toolchainv1alpha1.WorkspaceStatus{
					Namespaces: []toolchainv1alpha1.SpaceNamespace{
						{Name: "ns-dev"},
						{Name: "ns-stage"},
					},
				},
			},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "otherworkspace",
					},
					Status: toolchainv1alpha1.WorkspaceStatus{
						Namespaces: []toolchainv1alpha1.SpaceNamespace{
							{Name: "ns-test"},
						},
					},
				}},
			expectedErr: "",
		},
		"valid home workspace request": {
			requestedWorkspace: "", // home workspace is default when no workspace is specified
			requestedNamespace: "test-1234",
			workspaces: []toolchainv1alpha1.Workspace{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "homews",
				},
				Status: toolchainv1alpha1.WorkspaceStatus{
					Type: "home", // home workspace
					Namespaces: []toolchainv1alpha1.SpaceNamespace{
						{Name: "test-1234"},
					},
				},
			}},
			expectedErr: "",
		},
		"workspace not allowed": {
			requestedWorkspace: "notexist",
			requestedNamespace: "myns",
			workspaces: []toolchainv1alpha1.Workspace{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myworkspace",
				},
				Status: toolchainv1alpha1.WorkspaceStatus{
					Namespaces: []toolchainv1alpha1.SpaceNamespace{
						{Name: "ns-dev"},
					},
				},
			}},
			expectedErr: "access to workspace 'notexist' is forbidden",
		},
		"namespace not allowed": {
			requestedWorkspace: "myworkspace",
			requestedNamespace: "notexist",
			workspaces: []toolchainv1alpha1.Workspace{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myworkspace",
				},
				Status: toolchainv1alpha1.WorkspaceStatus{
					Namespaces: []toolchainv1alpha1.SpaceNamespace{
						{Name: "ns-dev"},
					},
				},
			}},
			expectedErr: "access to namespace 'notexist' in workspace 'myworkspace' is forbidden",
		},
		"namespace not allowed for home workspace": {
			requestedWorkspace: "", // home workspace is default when no workspace is specified
			requestedNamespace: "myns",
			workspaces: []toolchainv1alpha1.Workspace{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "homews",
				},
				Status: toolchainv1alpha1.WorkspaceStatus{
					Type: "home", // home workspace
					Namespaces: []toolchainv1alpha1.SpaceNamespace{
						{Name: "test-1234"}, // namespace does not match the requested one
					},
				},
			}},
			expectedErr: "access to namespace 'myns' in workspace 'homews' is forbidden",
		},
	}

	for k, tc := range tests {
		s.T().Run(k, func(t *testing.T) {
			err := validateWorkspaceRequest(tc.requestedWorkspace, tc.requestedNamespace, tc.workspaces)
			if tc.expectedErr == "" {
				require.NoError(s.T(), err)
			} else {
				require.EqualError(s.T(), err, tc.expectedErr)
			}
		})
	}
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
