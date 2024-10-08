package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	infservice "github.com/codeready-toolchain/registration-service/pkg/informers/service"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	rcontext "github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/codeready-toolchain/toolchain-common/pkg/test/space"
)

func TestListUserWorkspaces(t *testing.T) {
	tests := map[string]struct {
		username            string
		additionalSignups   []fake.SignupDef
		additionalObjects   []runtimeclient.Object
		expectedWorkspaces  func(*test.FakeClient) []toolchainv1alpha1.Workspace
		publicViewerEnabled bool
	}{
		"dancelover lists spaces with public-viewer enabled": {
			username: "dance.lover",
			additionalSignups: []fake.SignupDef{
				newSignup("communitylover", "community.lover", true),
			},
			additionalObjects: []runtimeclient.Object{
				fake.NewSpace("communitylover", "member-1", "communitylover", space.WithTierName("appstudio")),
				fake.NewSpaceBinding("communitylover-publicviewer", toolchainv1alpha1.KubesawAuthenticatedUsername, "communitylover", "viewer"),
			},
			expectedWorkspaces: func(fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
				return []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "communitylover", "viewer", false),
					workspaceFor(t, fakeClient, "dancelover", "admin", true),
					workspaceFor(t, fakeClient, "movielover", "other", false),
				}
			},
			publicViewerEnabled: true,
		},
		"dancelover lists spaces with public-viewer disabled": {
			username: "dance.lover",
			expectedWorkspaces: func(fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
				return []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "dancelover", "admin", true),
					workspaceFor(t, fakeClient, "movielover", "other", false),
				}
			},
			publicViewerEnabled: false,
		},
	}

	for k, tc := range tests {
		fakeSignupService, fakeClient := buildSpaceListerFakesWithResources(t, tc.additionalSignups, tc.additionalObjects)

		t.Run(k, func(t *testing.T) {
			// given
			signupProvider := fakeSignupService.GetSignupFromInformer

			proxyMetrics := metrics.NewProxyMetrics(prometheus.NewRegistry())

			s := &handlers.SpaceLister{
				GetSignupFunc: signupProvider,
				GetInformerServiceFunc: func() service.InformerService {
					return infservice.NewInformerService(fakeClient, test.HostOperatorNs)
				},
				ProxyMetrics: proxyMetrics,
			}

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)
			ctx.Set(rcontext.UsernameKey, tc.username)
			ctx.Set(rcontext.RequestReceivedTime, time.Now())

			// when
			ctx.Set(rcontext.PublicViewerEnabled, tc.publicViewerEnabled)
			ww, err := handlers.ListUserWorkspaces(ctx, s)

			// then
			require.NoError(t, err)
			// list workspace case
			expectedWs := tc.expectedWorkspaces(fakeClient)
			require.Equal(t, len(expectedWs), len(ww))
			for i, w := range ww {
				assert.Equal(t, expectedWs[i].Name, w.Name)
				assert.Equal(t, expectedWs[i].Status, w.Status)
			}
		})
	}
}

func TestHandleSpaceListRequest(t *testing.T) {
	tt := map[string]struct {
		publicViewerEnabled bool
	}{
		"public-viewer is enabled":  {publicViewerEnabled: true},
		"public-viewer is disabled": {publicViewerEnabled: false},
	}

	for k, rtc := range tt {

		t.Run(k, func(t *testing.T) {
			// given
			tests := map[string]struct {
				username           string
				expectedWs         func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace
				expectedErr        string
				expectedErrCode    int
				expectedWorkspace  string
				overrideSignupFunc func(ctx *gin.Context, userID, username string, checkUserSignupComplete bool) (*signup.Signup, error)
				mockFakeClient     func(fakeClient *test.FakeClient)
			}{
				"dancelover lists spaces": {
					username: "dance.lover",
					expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
						return []toolchainv1alpha1.Workspace{
							workspaceFor(t, fakeClient, "dancelover", "admin", true),
							workspaceFor(t, fakeClient, "movielover", "other", false),
						}
					},
					expectedErr: "",
				},
				"movielover lists spaces": {
					username: "movie.lover",
					expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
						return []toolchainv1alpha1.Workspace{
							workspaceFor(t, fakeClient, "movielover", "admin", true),
						}
					},
					expectedErr: "",
				},
				"signup has no compliant username set": {
					username:        "racing.lover",
					expectedWs:      nil,
					expectedErr:     "",
					expectedErrCode: 200,
				},
				"space not initialized yet": {
					username:        "panda.lover",
					expectedWs:      nil,
					expectedErr:     "",
					expectedErrCode: 200,
				},
				"no spaces found": {
					username:        "user.nospace",
					expectedWs:      nil,
					expectedErr:     "",
					expectedErrCode: 200,
				},
				"informer error": {
					username:        "dance.lover",
					expectedWs:      nil,
					expectedErr:     "list spacebindings error",
					expectedErrCode: 500,
					mockFakeClient: func(fakeClient *test.FakeClient) {
						fakeClient.MockList = func(ctx context.Context, list runtimeclient.ObjectList, opts ...runtimeclient.ListOption) error {
							if _, ok := list.(*toolchainv1alpha1.SpaceBindingList); ok {
								return fmt.Errorf("list spacebindings error")
							}
							return fakeClient.Client.List(ctx, list, opts...)
						}
					},
				},
				"get signup error": {
					username:        "dance.lover",
					expectedWs:      nil,
					expectedErr:     "signup error",
					expectedErrCode: 500,
					overrideSignupFunc: func(_ *gin.Context, _, _ string, _ bool) (*signup.Signup, error) {
						return nil, fmt.Errorf("signup error")
					},
				},
			}

			for k, tc := range tests {
				t.Run(k, func(t *testing.T) {
					// given
					fakeSignupService, fakeClient := buildSpaceListerFakes(t)
					if tc.mockFakeClient != nil {
						tc.mockFakeClient(fakeClient)
					}

					signupProvider := fakeSignupService.GetSignupFromInformer
					if tc.overrideSignupFunc != nil {
						signupProvider = tc.overrideSignupFunc
					}

					proxyMetrics := metrics.NewProxyMetrics(prometheus.NewRegistry())

					s := &handlers.SpaceLister{
						GetSignupFunc: signupProvider,
						GetInformerServiceFunc: func() service.InformerService {
							return infservice.NewInformerService(fakeClient, test.HostOperatorNs)
						},
						ProxyMetrics: proxyMetrics,
					}

					e := echo.New()
					req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
					rec := httptest.NewRecorder()
					ctx := e.NewContext(req, rec)
					ctx.Set(rcontext.UsernameKey, tc.username)
					ctx.Set(rcontext.RequestReceivedTime, time.Now())

					// when
					ctx.Set(rcontext.PublicViewerEnabled, rtc.publicViewerEnabled)
					err := handlers.HandleSpaceListRequest(s)(ctx)

					// then
					if tc.expectedErr != "" {
						// error case
						require.Equal(t, tc.expectedErrCode, rec.Code)
						require.Contains(t, rec.Body.String(), tc.expectedErr)
					} else {
						require.NoError(t, err)
						// list workspace case
						workspaceList, decodeErr := decodeResponseToWorkspaceList(rec.Body.Bytes())
						require.NoError(t, decodeErr)
						var expectedWorkspaces []toolchainv1alpha1.Workspace
						if tc.expectedWs != nil {
							expectedWorkspaces = tc.expectedWs(t, fakeClient)
						}
						require.Equal(t, len(expectedWorkspaces), len(workspaceList.Items))
						for i := range expectedWorkspaces {
							assert.Equal(t, expectedWorkspaces[i].Name, workspaceList.Items[i].Name)
							assert.Equal(t, expectedWorkspaces[i].Status, workspaceList.Items[i].Status)
						}
					}
				})
			}
		})
	}
}
