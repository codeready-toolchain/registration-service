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
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"
	"github.com/google/uuid"
	"go.uber.org/atomic"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	podsRequestURL := func(workspace string) string {
		return fmt.Sprintf("http://localhost:%s/workspaces/%s/api/pods", port, workspace)
	}

	podsInNamespaceRequestURL := func(workspace string, namespace string) string {
		return fmt.Sprintf("http://localhost:%s/workspaces/%s/api/namespaces/%s/pods", port, workspace, namespace)
	}

	s.Run("successfully proxy", func() {
		// user with public workspace
		smith := uuid.New()
		// user with private workspace
		alice := uuid.New()
		// unsigned user
		bob := uuid.New()
		// not ready
		john := uuid.New()
		// banned user
		eve, eveEmail := uuid.New(), "eve@somecorp.com"

		// Start the member-2 API Server
		httpTestServerResponse := "my response"
		testServer := httptest.NewServer(nil)
		defer testServer.Close()

		// initialize SignupService
		signupService := fake.NewSignupService(
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
			fake.Signup(john.String(), &signup.Signup{
				Name:              "john",
				CompliantUsername: "john",
				Username:          "john@",
				Status: signup.Status{
					Ready: false,
				},
			}),
			fake.Signup(eve.String(), &signup.Signup{
				Name:              "eve",
				CompliantUsername: "eve",
				Username:          "eve@",
				Status: signup.Status{
					Ready:  false,
					Reason: toolchainv1alpha1.UserSignupUserBannedReason,
				},
			}),
		)

		// init fakeClient
		sbSmithCommunitySmith := fake.NewSpaceBinding("smith-community-smith", "smith", "smith-community", "admin")
		commSpacePublicViewer := fake.NewSpaceBinding("smith-community-publicviewer", toolchainv1alpha1.KubesawAuthenticatedUsername, "smith-community", "viewer")
		alicePrivate := fake.NewSpaceBinding("alice-default", "alice", "alice-private", "admin")
		cli := fake.InitClient(s.T(), sbSmithCommunitySmith, commSpacePublicViewer, alicePrivate)

		// configure informer
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
		inf.ListBannedUsersByEmailFunc = func(email string) ([]toolchainv1alpha1.BannedUser, error) {
			switch email {
			case eveEmail:
				return []toolchainv1alpha1.BannedUser{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "eve",
							Namespace: "toolchain-host-operator",
						},
						Spec: toolchainv1alpha1.BannedUserSpec{},
					},
				}, nil
			default:
				return nil, nil
			}
		}

		// configure fakeApp
		fakeApp.Err = nil
		fakeApp.MemberClusterServiceMock = s.newMemberClusterServiceWithMembers(testServer.URL)
		fakeApp.SignupServiceMock = signupService
		fakeApp.InformerServiceMock = inf

		// configure Application
		s.Application.MockSignupService(signupService)
		s.Application.MockInformerService(inf)

		// configure proxy
		p.spaceLister = &handlers.SpaceLister{
			GetSignupFunc: fakeApp.SignupServiceMock.GetSignupFromInformer,
			GetInformerServiceFunc: func() appservice.InformerService {
				return inf
			},
			ProxyMetrics: p.metrics,
		}

		// run test cases
		tests := map[string]struct {
			ProxyRequestMethod              string
			ProxyRequestHeaders             http.Header
			ExpectedAPIServerRequestHeaders http.Header
			ExpectedProxyResponseStatus     int
			RequestPath                     string
			ExpectedResponse                string
		}{
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
				RequestPath:                 podsRequestURL("smith-community"),
				ExpectedResponse:            httpTestServerResponse,
			},
			// Given The not ready user john exists
			// When  john requests the list of pods in workspace smith-community
			// Then  the request is forwarded from the proxy
			// And   the request impersonates john
			// And   the request's X-SSO-User Header is set to john's ID
			// And   the request is successful
			"plain http actual request as notReadyUser": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(john)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {toolchainv1alpha1.KubesawAuthenticatedUsername},
					"X-SSO-User":       {"username-" + john.String()},
				},
				ExpectedProxyResponseStatus: http.StatusOK,
				RequestPath:                 podsRequestURL("smith-community"),
				ExpectedResponse:            httpTestServerResponse,
			},
			// Given The not signed up user bob exists
			// When  bob requests the list of pods in workspace smith-community
			// Then  the request is forwarded from the proxy
			// And   the request impersonates bob
			// And   the request's X-SSO-User Header is set to bob's ID
			// And   the request is successful
			"plain http actual request as not signed up user": {
				ProxyRequestMethod:  "GET",
				ProxyRequestHeaders: map[string][]string{"Authorization": {"Bearer " + s.token(bob)}},
				ExpectedAPIServerRequestHeaders: map[string][]string{
					"Authorization":    {"Bearer clusterSAToken"},
					"Impersonate-User": {toolchainv1alpha1.KubesawAuthenticatedUsername},
					"X-SSO-User":       {"username-" + bob.String()},
				},
				ExpectedProxyResponseStatus: http.StatusOK,
				RequestPath:                 podsRequestURL("smith-community"),
				ExpectedResponse:            httpTestServerResponse,
			},
			// Given smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// And   a user named alice exists
			// And   smith's smith-community is not directly shared with alice
			// When  alice requests the list of pods in workspace smith-community
			// Then  the request is forwarded from the proxy
			// And   the request impersonates the PublicViewer
			// And   the request's X-SSO-User Header is set to alice's ID
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
				RequestPath:                 podsRequestURL("smith-community"),
				ExpectedResponse:            httpTestServerResponse,
			},
			// Given user alice exists
			// And   alice owns a private workspace
			// When  smith requests the list of pods in alice's workspace
			// Then  the proxy does NOT forward the request
			// And   the proxy rejects the call with 403 Forbidden
			"plain http actual request as non-owner to private workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(smith)}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsRequestURL("alice-private"),
				ExpectedResponse:            "invalid workspace request: access to workspace 'alice-private' is forbidden",
			},
			// Given banned user eve exists
			// And   user smith exists
			// And   smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// When  eve requests the list of pods in smith's workspace
			// Then  the proxy does NOT forward the request
			// And   the proxy rejects the call with 403 Forbidden
			"plain http actual request as banned user to community workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(eve, authsupport.WithEmailClaim(eveEmail))}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsRequestURL("smith-community"),
				ExpectedResponse:            "user access is forbidden: user access is forbidden",
			},
			// Given banned user eve exist
			// And   user alice exists
			// And   alice owns a private workspace
			// When  eve requests the list of pods in alice's workspace
			// Then  the proxy does NOT forward the request
			// And   the proxy rejects the call with 403 Forbidden
			"plain http actual request as banned user to private workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(eve, authsupport.WithEmailClaim(eveEmail))}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsRequestURL("alice-private"),
				ExpectedResponse:            "user access is forbidden: user access is forbidden",
			},
			// Given user alice exists
			// And   alice owns a private workspace
			// When  alice requests the list of pods in a non existing namespace in alice's workspace
			// Then  the proxy does NOT forward the request
			// And   the proxy rejects the call with 403 Forbidden
			"plain http request as owner to not existing namespace in private workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(alice)}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsInNamespaceRequestURL("alice-private", "not-existing"),
				ExpectedResponse:            "invalid workspace request: access to namespace 'not-existing' in workspace 'alice-private' is forbidden",
			},
			// Given smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// When  smith requests the list of pods in a non existing namespace in workspace smith-community
			// Then  the request is forwarded from the proxy
			// And   the request impersonates smith
			// And   the request's X-SSO-User Header is set to smith's ID
			// And   the request is successful
			"plain http request as owner to not existing namespace in community workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(smith)}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsInNamespaceRequestURL("smith-community", "not-existing"),
				ExpectedResponse:            "invalid workspace request: access to namespace 'not-existing' in workspace 'smith-community' is forbidden",
			},
			// Given smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// And   user alice exists
			// When  alice requests the list of pods in a non existing namespace in smith's workspace
			// Then  the proxy does NOT forward the request
			// And   the proxy rejects the call with 403 Forbidden
			"plain http request as community user to not existing namespace in community workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(alice)}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsInNamespaceRequestURL("smith-community", "not-existing"),
				ExpectedResponse:            "invalid workspace request: access to namespace 'not-existing' in workspace 'smith-community' is forbidden",
			},
			// Given smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// When  bob requests the list of pods in a non existing namespace in smith's workspace
			// Then  the proxy does NOT forward the request
			// And   the proxy rejects the call with 403 Forbidden
			"plain http request as unsigned user to not existing namespace in community workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(bob)}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsInNamespaceRequestURL("smith-community", "not-existing"),
				ExpectedResponse:            "invalid workspace request: access to namespace 'not-existing' in workspace 'smith-community' is forbidden",
			},
			// Given smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// And   not ready user john exists
			// When  john requests the list of pods in a non existing namespace in smith's workspace
			// Then  the proxy does NOT forward the request
			// And   the proxy rejects the call with 403 Forbidden
			"plain http request as notReadyUser to not existing namespace in community workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(john)}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsInNamespaceRequestURL("smith-community", "not-existing"),
				ExpectedResponse:            "invalid workspace request: access to namespace 'not-existing' in workspace 'smith-community' is forbidden",
			},
			// Given banned user eve exists
			// And   user smith exists
			// And   smith owns a workspace named smith-community
			// And   smith-community is publicly visible (shared with PublicViewer)
			// When  eve requests the list of pods in a non existing namespace smith's workspace
			// Then  the proxy does NOT forward the request
			// And   the proxy rejects the call with 403 Forbidden
			"plain http actual request as banned user to not existing namespace community workspace": {
				ProxyRequestMethod:          "GET",
				ProxyRequestHeaders:         map[string][]string{"Authorization": {"Bearer " + s.token(eve, authsupport.WithEmailClaim(eveEmail))}},
				ExpectedProxyResponseStatus: http.StatusForbidden,
				RequestPath:                 podsInNamespaceRequestURL("smith-community", "not-existing"),
				ExpectedResponse:            "user access is forbidden: user access is forbidden",
			},
		}

		for k, tc := range tests {
			s.Run(k, func() {
				testServerInvoked := atomic.NewBool(false)

				// given
				testServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					v := testServerInvoked.Swap(true)
					require.False(s.T(), v, "expected handler to be invoked just one time")

					w.Header().Set("Content-Type", "application/json")
					// Set the Access-Control-Allow-Origin header to make sure it's overridden by the proxy response modifier
					w.Header().Set("Access-Control-Allow-Origin", "dummy")
					w.WriteHeader(http.StatusOK)
					_, err := w.Write([]byte(httpTestServerResponse))
					require.NoError(s.T(), err)
					for hk, hv := range tc.ExpectedAPIServerRequestHeaders {
						require.Len(s.T(), r.Header.Values(hk), len(hv))
						for i := range hv {
							assert.Equal(s.T(), hv[i], r.Header.Values(hk)[i], "header %s", hk)
						}
					}
				})

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

				forwardExpected := len(tc.ExpectedAPIServerRequestHeaders) > 0
				requestForwarded := testServerInvoked.Load()
				require.Equal(s.T(),
					forwardExpected, requestForwarded,
					"expecting call forward to be %v, got %v", forwardExpected, requestForwarded,
				)
			})
		}
	})
}
