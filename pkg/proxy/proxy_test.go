package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
				defer resp.Body.Close()
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
					defer resp.Body.Close()
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
					defer resp.Body.Close()
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
					defer resp.Body.Close()
					assert.Equal(s.T(), http.StatusUnauthorized, resp.StatusCode)
					s.assertResponseBody(resp, "invalid bearer token: unable to extract userID from token: token does not comply to expected claims: subject missing")
				})

				s.Run("unauthorized if workspace context is invalid", func() {
					// when
					req := s.request()
					req.URL.Path = "http://localhost:8081/workspaces/myworkspace" // invalid workspace context
					require.NotNil(s.T(), req)

					// when
					resp, err := http.DefaultClient.Do(req)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), resp)
					defer resp.Body.Close()
					assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
					s.assertResponseBody(resp, "unable to get workspace context: workspace request path has too few segments '/workspaces/myworkspace'; expected path format: /workspaces/<workspace_name>/api/...")
				})

				s.Run("internal error if get accesses returns an error", func() {
					// given
					req := s.request()
					fakeApp.Accesses = map[string]*access.ClusterAccess{}
					fakeApp.Err = errors.New("some-error")

					// when
					resp, err := http.DefaultClient.Do(req)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), resp)
					defer resp.Body.Close()
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
						defer resp.Body.Close()
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
					"proxy plain http actual request": {
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
					"plain http upgrade POST request": {
						ProxyRequestMethod: "POST",
						ProxyRequestHeaders: map[string][]string{
							"Authorization": {"Bearer " + s.token(userID)},
							"Connection":    {"Upgrade"},
							"Upgrade":       {"SPDY/3.1"},
						},
						ExpectedAPIServerRequestHeaders: map[string][]string{
							"Authorization":    {"Bearer clusterSAToken"},
							"Impersonate-User": {"smith2"},
							"Connection":       {"Upgrade"},
							"Upgrade":          {"SPDY/3.1"},
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

				rejectedHeaders := []headerToAdd{
					{},
					{"impersonate-user", "myvalue"},
					{"Impersonate-User", "myvalue"},
					{"Impersonate-Group", "developers"},
					{"Impersonate-gRoup", "admins"},
					{"Impersonate-Extra-dn", "cn=jane,ou=engineers,dc=example,dc=com"},
					{"Impersonate-Extra-acme.com%2Fproject", "some-project"},
					{"Impersonate-Extra-scopes", "view"},
					{"Impersonate-Extra-scopes", "development"},
					{"Impersonate-Uid", "06f6ce97-e2c5-4ab8-7ba5-7654dd08d52b"},
					{"Impersonate-New", "myvalue"},
				}

				for k, tc := range tests {
					s.Run(k, func() {

						for _, firstHeader := range rejectedHeaders {
							rejectedHeadersToAdd := []headerToAdd{firstHeader}
							for _, additionalHeader := range rejectedHeaders {
								rejectedHeadersToAdd = append(rejectedHeadersToAdd, additionalHeader)

								// Test each request using both the default workspace URL and a URL that uses the
								// workspace context. Both should yield the same results.
								for workspaceContext, reqPath := range map[string]string{
									"default workspace":    "http://localhost:8081/api/mycoolworkspace/pods",
									"workspace context":    "http://localhost:8081/workspaces/mycoolworkspace/api/mycoolworkspace/pods",
									"proxy plugin context": "http://localhost:8081/plugins/myplugin/workspaces/mycoolworkspace/api/mycoolworkspace/pods",
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
										for _, header := range rejectedHeadersToAdd {
											if header.key != "" {
												req.Header.Add(header.key, header.value)
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
												impersonateUser := tc.ExpectedAPIServerRequestHeaders.Get("Impersonate-User")
												for _, rejectedHeader := range rejectedHeaders {
													if impersonateUser != "" && strings.ToLower(rejectedHeader.key) == "impersonate-user" { // only the expected Impersonate-User header should not be rejected
														assert.NotEqual(s.T(), rejectedHeader.value, r.Header.Get(rejectedHeader.key))
													} else {
														assert.Emptyf(s.T(), r.Header.Get(rejectedHeader.key), "The header %s should be deleted", rejectedHeader.key)
														assert.Emptyf(s.T(), r.Header.Values(rejectedHeader.key), "The header %s should be deleted", rejectedHeader.key)
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
											inf.GetProxyPluginConfigFunc = func(name string) (*toolchainv1alpha1.ProxyPlugin, error) {
												switch name {
												case "myplugin":
													return &toolchainv1alpha1.ProxyPlugin{
														ObjectMeta: metav1.ObjectMeta{
															Namespace: metav1.NamespaceDefault,
															Name:      "myplugin",
														},
														Spec: toolchainv1alpha1.ProxyPluginSpec{
															OpenShiftRouteTargetEndpoint: &toolchainv1alpha1.OpenShiftRouteTarget{
																Namespace: metav1.NamespaceDefault,
																Name:      metav1.NamespaceDefault,
															},
														},
														Status: toolchainv1alpha1.ProxyPluginStatus{},
													}, nil
												}
												return nil, fmt.Errorf("proxy plugin not found")
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
										defer resp.Body.Close()
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
							}
						}
					})
				}
			})

		})
	}
}

type headerToAdd struct {
	key, value string
}

func (s *TestProxySuite) newMemberClusterServiceWithMembers(serverURL string) appservice.MemberClusterService {
	fakeClient := commontest.NewFakeClient(s.T())
	serverHost := serverURL
	switch {
	case strings.HasPrefix(serverURL, "http://"):
		serverHost = strings.TrimPrefix(serverURL, "http://")
	case strings.HasPrefix(serverURL, "https://"):
		serverHost = strings.TrimPrefix(serverURL, "https://")
	}
	fakeClient.MockGet = func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		route, ok := obj.(*routev1.Route)
		if ok && key.Namespace == metav1.NamespaceDefault && key.Name == metav1.NamespaceDefault {
			route.Namespace = key.Namespace
			route.Name = key.Name
			route.Spec.Port = &routev1.RoutePort{TargetPort: intstr.FromString("http")}
			route.Status.Ingress = []routev1.RouteIngress{
				{
					Host: serverHost,
				},
			}
			return nil
		}
		return fakeClient.Client.Get(ctx, key, obj, opts...)
	}
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
						Client: fakeClient,
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
		expectedPlugin    string
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
		"no workspace context but plugins in kube api portion": {
			path:              "/api/plugins/something",
			expectedWorkspace: "",
			expectedPath:      "/api/plugins/something",
			expectedErr:       "",
		},
		"workspace instead of workspaces": {
			path:              "/workspace/myworkspace/api",
			expectedWorkspace: "",
			expectedPath:      "/workspace/myworkspace/api",
			expectedErr:       "",
		},
		"valid workspace context with plugin": {
			path:              "/plugins/tekton-results/workspaces/myworkspace/api",
			expectedWorkspace: "myworkspace",
			expectedPath:      "/api",
			expectedErr:       "",
			expectedPlugin:    "tekton-results",
		},
		"valid workspace context with plugin plus another plugin in kube api portion": {
			path:              "/plugins/tekton-results/workspaces/myworkspace/api/plugins/something",
			expectedWorkspace: "myworkspace",
			expectedPath:      "/api/plugins/something",
			expectedErr:       "",
			expectedPlugin:    "tekton-results",
		},
		"no specific plugin segment no trailing slash": {
			path:              "/plugins",
			expectedWorkspace: "",
			expectedPath:      "/plugins",
			expectedErr:       "",
			expectedPlugin:    "",
		},
		"no specific plugin segment with trailing slash": {
			path:              "/plugins/",
			expectedWorkspace: "",
			expectedPath:      "/plugins/",
			expectedErr:       "path \"/plugins/\" not a proxied route request",
			expectedPlugin:    "",
		},
		"plugin spec but nothing else": {
			path:              "/plugins/whatever",
			expectedWorkspace: "",
			expectedPath:      "",
			expectedErr:       "",
			expectedPlugin:    "whatever",
		},
		"valid workspace context with route": {
			path:              "/plugins/tekton-results/workspaces/myworkspace",
			expectedWorkspace: "myworkspace",
			expectedPath:      "",
			expectedErr:       "",
			expectedPlugin:    "tekton-results",
		},
		"invalid workspace context with route": {
			path:              "/plugins/tekton-results/workspaces/",
			expectedWorkspace: "",
			expectedPath:      "/workspaces/",
			expectedErr:       "workspace request path has too few segments '/workspaces/'; expected path format: /workspaces/<workspace_name>/<optional path>",
			expectedPlugin:    "",
		},
		"plugin and workspaces as the sub path": {
			path:              "/plugins/tekton-results/workspaces",
			expectedWorkspace: "",
			expectedPath:      "/workspaces",
			expectedErr:       "",
			expectedPlugin:    "tekton-results",
		},
		"no workspace context with route": {
			path:              "/plugins/tekton-results/api/pods",
			expectedWorkspace: "",
			expectedPath:      "/api/pods",
			expectedErr:       "",
			expectedPlugin:    "tekton-results",
		},
		"workspace instead of workspaces with route": {
			path:              "/plugins/tekton-results/workspace/myworkspace/api",
			expectedWorkspace: "",
			expectedPath:      "/workspace/myworkspace/api",
			expectedErr:       "",
			expectedPlugin:    "tekton-results",
		},
	}

	for k, tc := range tests {
		s.T().Run(k, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					Path: tc.path,
				},
			}
			proxy, workspace, err := getWorkspaceContext(req)
			if tc.expectedErr == "" {
				require.NoError(s.T(), err, fmt.Sprintf("failed for tc %s", k))
			} else {
				require.EqualError(s.T(), err, tc.expectedErr, fmt.Sprintf("failed for tc %s", k))
			}
			assert.Equal(s.T(), tc.expectedWorkspace, workspace, fmt.Sprintf("failed for tc %s", k))
			assert.Equal(s.T(), tc.expectedPath, req.URL.Path, fmt.Sprintf("failed for tc %s", k))
			assert.Equal(s.T(), tc.expectedPlugin, proxy, fmt.Sprintf("failed for tc %s", k))
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

func (s *TestProxySuite) TestGetTransport() {

	s.T().Run("when not prod", func(t *testing.T) {
		for _, envName := range []testconfig.EnvName{testconfig.E2E, testconfig.Dev} {
			s.T().Run("env "+string(envName), func(t *testing.T) {
				// given
				env := s.DefaultConfig().Environment()
				defer s.SetConfig(testconfig.RegistrationService().
					Environment(env))
				s.SetConfig(testconfig.RegistrationService().
					Environment(string(envName)))

				// when
				transport := getTransport(map[string][]string{})

				// then
				expectedTransport := &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true, // nolint:gosec
					},
				}
				assert.Equal(t, expectedTransport, transport)
			})
		}
	})

	s.T().Run("for prod", func(t *testing.T) {
		// given
		env := s.DefaultConfig().Environment()
		defer s.SetConfig(testconfig.RegistrationService().
			Environment(env))
		s.SetConfig(testconfig.RegistrationService().
			Environment(string(testconfig.Prod)))

		s.T().Run("upgrade header is set to 'SPDY/3.1'", func(t *testing.T) {
			// when
			transport := getTransport(map[string][]string{
				"Connection": {"Upgrade"},
				"Upgrade":    {"SPDY/3.1"},
			})

			// then
			tr := transport.(*http.Transport)
			assert.Equal(t, []string{"http/1.1"}, tr.TLSClientConfig.NextProtos)
			assert.False(t, tr.ForceAttemptHTTP2)
			assert.NotEqual(t, http.DefaultTransport, transport)
		})

		s.T().Run("upgrade header is set to 'websocket'", func(t *testing.T) {
			// when
			transport := getTransport(map[string][]string{
				"Connection": {"Upgrade"},
				"Upgrade":    {"websocket"},
			})

			// then
			assert.Equal(t, http.DefaultTransport, transport)
		})

		s.T().Run("no upgrade header is set", func(t *testing.T) {
			// when
			transport := getTransport(map[string][]string{})

			// then
			assert.Equal(t, http.DefaultTransport, transport)
		})
	})
}

func (s *TestProxySuite) request() *http.Request {
	req, err := http.NewRequest("GET", "http://localhost:8081/api/mycoolworkspace/pods", nil)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), req)
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	req.Header.Set("Authorization", "Bearer "+s.token(userID))

	return req
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
