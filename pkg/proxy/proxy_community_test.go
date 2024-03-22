package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	appservice "github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *TestProxySuite) TestProxyCommunity() {
	// given

	port := "30456"

	env := s.DefaultConfig().Environment()
	defer s.SetConfig(testconfig.RegistrationService().
		Environment(env))
	s.SetConfig(testconfig.RegistrationService().
		Environment(string(testconfig.E2E))) // We use e2e-test environment just to be able to re-use token generation
	_, err := auth.InitializeDefaultTokenParser()
	require.NoError(s.T(), err)

	cfg := commonconfig.PublicViewerConfig{
		Config: toolchainv1alpha1.PublicViewerConfig{
			Enabled:  true,
			Username: "public-viewer",
		},
	}
	for _, environment := range []testconfig.EnvName{testconfig.E2E, testconfig.Dev, testconfig.Prod} {
		s.Run("for environment "+string(environment), func() {
			// spin up proxy
			s.SetConfig(testconfig.RegistrationService().
				Environment(string(environment)))
			fakeApp := &fake.ProxyFakeApp{}
			p, server := s.spinUpProxy(fakeApp, cfg, port)
			defer func() {
				_ = server.Close()
			}()

			// wait for proxy to be alive
			s.Run("is alive", func() {
				s.waitForProxyToBeAlive(port)
			})
			s.Run("health check ok", func() {
				s.checkProxyIsHealthy(port)
			})

			// run community tests
			s.checkProxyCommunityOK(fakeApp, p, port, &cfg)
		})
	}
}

func (s *TestProxySuite) checkProxyCommunityOK(fakeApp *fake.ProxyFakeApp, p *Proxy, port string, publicViewerConfig *commonconfig.PublicViewerConfig) {
	s.Run("successfully proxy", func() {
		owner, err := uuid.NewV4()
		require.NoError(s.T(), err)
		communityUser, err := uuid.NewV4()
		require.NoError(s.T(), err)

		// Start the member-2 API Server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Set the Access-Control-Allow-Origin header to make sure it's overridden by the proxy response modifier
			w.Header().Set("Access-Control-Allow-Origin", "dummy")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("my response"))
			require.NoError(s.T(), err)
		}))
		defer testServer.Close()

		type testCase struct {
			ProxyRequestMethod              string
			ProxyRequestHeaders             http.Header
			ExpectedAPIServerRequestHeaders http.Header
			ExpectedProxyResponseStatus     int
			RequestPath                     string
		}

		tests := map[string]testCase{
			"plain http actual request as owner": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(owner)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {"smith2"},
				},
				ExpectedProxyResponseStatus: http.StatusOK,
				RequestPath:                 fmt.Sprintf("http://localhost:%s/workspaces/communityspace/api/communityspace/pods", port),
			},
			"plain http actual request as community user": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(communityUser)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {publicViewerConfig.Username()},
					"SSO-User":         {"username-" + communityUser.String()},
				},
				ExpectedProxyResponseStatus: http.StatusOK,
				RequestPath:                 fmt.Sprintf("http://localhost:%s/workspaces/communityspace/api/communityspace/pods", port),
			},
		}

		for k, tc := range tests {
			s.Run(k, func() {

				// given
				fakeApp.Err = nil

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
							assert.Equal(s.T(), hv[i], r.Header.Values(hk)[i], "header %s", hk)
						}
					}
				})
				fakeApp.SignupServiceMock = fake.NewSignupService(
					fake.Signup(owner.String(), &signup.Signup{
						Name:              "smith2",
						APIEndpoint:       testServer.URL,
						ClusterName:       "member-2",
						CompliantUsername: "smith2",
						Username:          "smith2@",
						Status: signup.Status{
							Ready: true,
						},
					}),
					fake.Signup(communityUser.String(), &signup.Signup{
						Name:              "communityUser",
						APIEndpoint:       testServer.URL,
						ClusterName:       "member-2",
						CompliantUsername: "communityuser",
						Username:          "communityUser@",
						Status: signup.Status{
							Ready: true,
						},
					}),
				)
				s.Application.MockSignupService(fakeApp.SignupServiceMock)
				inf := fake.NewFakeInformer()
				inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
					switch name {
					case "communityspace":
						return fake.NewSpace("communityspace", "member-2", "smith2"), nil
					}
					return nil, fmt.Errorf("space not found error")
				}

				sbmycoolSmith2 := fake.NewSpaceBinding("communityspace-smith2", "smith2", "communityspace", "admin")
				commSpacePublicViewer := fake.NewSpaceBinding("communityspace-publicviewer", p.publicViewerConfig.Username(), "communityspace", "viewer")

				cli := fake.InitClient(s.T(), sbmycoolSmith2, commSpacePublicViewer)
				inf.ListSpaceBindingFunc = func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
					sbs := toolchainv1alpha1.SpaceBindingList{}
					opts := &client.ListOptions{
						LabelSelector: labels.NewSelector().Add(reqs...),
					}
					log.Printf("received reqs: %v", reqs)
					if err := cli.Client.List(context.TODO(), &sbs, opts); err != nil {
						return nil, err
					}
					log.Printf("returning sbs: %v", sbs.Items)
					return sbs.Items, nil
				}
				inf.GetProxyPluginConfigFunc = func(name string) (*toolchainv1alpha1.ProxyPlugin, error) {
					return nil, fmt.Errorf("proxy plugin not found")
				}
				inf.GetNSTemplateTierFunc = func(_ string) (*toolchainv1alpha1.NSTemplateTier, error) {
					return fake.NewBase1NSTemplateTier(), nil
				}
				s.Application.MockInformerService(inf)
				fakeApp.MemberClusterServiceMock = s.newMemberClusterServiceWithMembers(testServer.URL, publicViewerConfig)
				fakeApp.InformerServiceMock = inf

				p.spaceLister = &handlers.SpaceLister{
					GetSignupFunc: fakeApp.SignupServiceMock.GetSignupFromInformer,
					GetInformerServiceFunc: func() appservice.InformerService {
						return inf
					},
					ProxyMetrics: p.metrics,
				}

				// prepare request
				req, err := http.NewRequest(tc.ProxyRequestMethod, tc.RequestPath, nil)
				require.NoError(s.T(), err)
				require.NotNil(s.T(), req)

				for hk, hv := range tc.ProxyRequestHeaders {
					for _, v := range hv {
						req.Header.Add(hk, v)
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
				s.assertResponseBody(resp, "my response")
			})
		}
	})
}
