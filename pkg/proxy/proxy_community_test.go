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
	"github.com/google/uuid"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *TestProxySuite) TestProxyCommunityEnabled() {
	// given

	port := "30456"

	env := s.DefaultConfig().Environment()
	defer s.SetConfig(testconfig.RegistrationService().
		Environment(env))
	s.SetConfig(testconfig.RegistrationService().
		Environment(string(testconfig.E2E))) // We use e2e-test environment just to be able to re-use token generation
	_, err := auth.InitializeDefaultTokenParser()
	require.NoError(s.T(), err)

	for _, environment := range []testconfig.EnvName{testconfig.E2E, testconfig.Dev, testconfig.Prod} {
		s.Run("for environment "+string(environment), func() {
			// spin up proxy
			s.SetConfig(
				testconfig.RegistrationService().Environment(string(environment)),
				testconfig.PublicViewerConfig(true),
			)
			fakeApp := &fake.ProxyFakeApp{}
			p, server := s.spinUpProxy(fakeApp, port)
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
			s.checkProxyCommunityOK(fakeApp, p, port)
		})
	}
}

func (s *TestProxySuite) checkProxyCommunityOK(fakeApp *fake.ProxyFakeApp, p *Proxy, port string) {
	s.Run("successfully proxy", func() {
		smith := uuid.New()
		alice := uuid.New()
		notReadyUser := uuid.New()
		notSignedUpUser := uuid.New()
		httpTestServerResponse := "my response"

		// Start the member-2 API Server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Set the Access-Control-Allow-Origin header to make sure it's overridden by the proxy response modifier
			w.Header().Set("Access-Control-Allow-Origin", "dummy")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(httpTestServerResponse))
			require.NoError(s.T(), err)
		}))
		defer testServer.Close()

		type testCase struct {
			ProxyRequestMethod              string
			ProxyRequestHeaders             http.Header
			ExpectedAPIServerRequestHeaders http.Header
			ExpectedProxyResponseStatus     int
			RequestPath                     string
			ExpectedResponse                string
		}

		podsRequestUrl := func(workspace string) string {
			return fmt.Sprintf("http://localhost:%s/workspaces/%s/api/pods", port, workspace)
		}

		tests := map[string]testCase{
			// Given smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// When  smith requests the list of pods in workspace smith-community
			// Then  the request is forwarded from the proxy
			// And   the request impersonates smith
			// And   the request's X-SSO-User Header is set to smith's ID
			// And   the request is successful
			"plain http actual request as community space owner": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(smith)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {"smith"},
					"X-SSO-User":       {"username-" + smith.String()},
				},
				ExpectedProxyResponseStatus: http.StatusOK,
				RequestPath:                 podsRequestUrl("smith-community"),
				ExpectedResponse:            httpTestServerResponse,
			},
			// Given A not ready user exists
			// When  the not ready user requests the list of pods in workspace smith-community
			// Then  the request is forwarded from the proxy
			// And   the request impersonates the not ready user
			// And   the request's X-SSO-User Header is set to not ready user's ID
			// And   the request is successful
			"plain http actual request as notReadyUser": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(notReadyUser)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {toolchainv1alpha1.KubesawAuthenticatedUsername},
					"X-SSO-User":       {"username-" + notReadyUser.String()},
				},
				ExpectedProxyResponseStatus: http.StatusOK,
				RequestPath:                 podsRequestUrl("smith-community"),
				ExpectedResponse:            httpTestServerResponse,
			},
			// Given A not signed up user exists
			// When  the not signed up user requests the list of pods in workspace smith-community
			// Then  the request is forwarded from the proxy
			// And   the request impersonates the not signed up user
			// And   the request's X-SSO-User Header is set to not signed up user's ID
			// And   the request is successful
			"plain http actual request as notSignedUpUser": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(notSignedUpUser)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {toolchainv1alpha1.KubesawAuthenticatedUsername},
					"X-SSO-User":       {"username-" + notSignedUpUser.String()},
				},
				ExpectedProxyResponseStatus: http.StatusOK,
				RequestPath:                 podsRequestUrl("smith-community"),
				ExpectedResponse:            httpTestServerResponse,
			},
			// Given smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// And   a user named communityuser exists
			// And   smith's smith-community is not directly shared with communityuser
			// When  communityuser requests the list of pods in workspace smith-community
			// Then  the request is forwarded from the proxy
			// And   the request impersonates the PublicViewer
			// And   the request's X-SSO-User Header is set to communityuser's ID
			// And   the request is successful
			"plain http actual request as community user": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(alice)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {toolchainv1alpha1.KubesawAuthenticatedUsername},
					"X-SSO-User":       {"username-" + alice.String()},
				},
				ExpectedProxyResponseStatus: http.StatusOK,
				RequestPath:                 podsRequestUrl("smith-community"),
				ExpectedResponse:            httpTestServerResponse,
			},
			// Given user alice exists
			// And   alice owns a private workspace
			// When  smith requests the list of pods in alice's workspace
			// Then  the request is forwarded from the proxy
			// And   the request impersonates smith
			// And   the request's X-SSO-User Header is set to smith's ID
			// And   the request is NOT successful
			"plain http actual request as non-owner to private workspace": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(smith)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {"smith"},
					"X-SSO-User":       {"username-" + smith.String()},
				},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsRequestUrl("alice-private"),
				ExpectedResponse:            "invalid workspace request: access to workspace 'alice-private' is forbidden",
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
					fake.Signup(smith.String(), &signup.Signup{
						Name:              "smith",
						APIEndpoint:       testServer.URL,
						ClusterName:       "member-2",
						CompliantUsername: "smith",
						Username:          "smith@",
						Status: signup.Status{
							Ready: true,
						},
					}),
					fake.Signup(alice.String(), &signup.Signup{
						Name:              "alice",
						APIEndpoint:       testServer.URL,
						ClusterName:       "member-2",
						CompliantUsername: "alice",
						Username:          "alice@",
						Status: signup.Status{
							Ready: true,
						},
					}),
					fake.Signup(notReadyUser.String(), &signup.Signup{
						Name:              "notReadyUser",
						CompliantUsername: "notReadyUser",
						Username:          "notReadyUser@",
						Status: signup.Status{
							Ready: false,
						},
					}),
				)
				s.Application.MockSignupService(fakeApp.SignupServiceMock)
				inf := fake.NewFakeInformer()
				inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
					switch name {
					case "smith-community":
						return fake.NewSpace("smith-community", "member-2", "smith"), nil
					case "alice-private":
						return fake.NewSpace("alice-private", "member-2", "alice"), nil
					}
					return nil, fmt.Errorf("space not found error")
				}

				sbSmithCommunitySmith := fake.NewSpaceBinding("smith-community-smith", "smith", "smith-community", "admin")
				commSpacePublicViewer := fake.NewSpaceBinding("smith-community-publicviewer", toolchainv1alpha1.KubesawAuthenticatedUsername, "smith-community", "viewer")
				alicePrivate := fake.NewSpaceBinding("alice-default", "alice", "alice-private", "admin")

				cli := fake.InitClient(s.T(), sbSmithCommunitySmith, commSpacePublicViewer, alicePrivate)
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
				inf.GetProxyPluginConfigFunc = func(_ string) (*toolchainv1alpha1.ProxyPlugin, error) {
					return nil, fmt.Errorf("proxy plugin not found")
				}
				inf.GetNSTemplateTierFunc = func(_ string) (*toolchainv1alpha1.NSTemplateTier, error) {
					return fake.NewBase1NSTemplateTier(), nil
				}
				s.Application.MockInformerService(inf)
				fakeApp.MemberClusterServiceMock = s.newMemberClusterServiceWithMembers(testServer.URL)
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
				s.assertResponseBody(resp, tc.ExpectedResponse)
			})
		}
	})
}
