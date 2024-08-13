package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	rcontext "github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/codeready-toolchain/toolchain-common/pkg/test/space"
)

func TestListUserWorkspaces(t *testing.T) {
	tests := map[string]struct {
		username            string
		additionalSignups   []fake.SignupDef
		additionalObjects   []client.Object
		expectedWorkspaces  func(*test.FakeClient) []toolchainv1alpha1.Workspace
		publicViewerEnabled bool
	}{
		"dancelover lists spaces with public-viewer enabled": {
			username: "dance.lover",
			additionalSignups: []fake.SignupDef{
				newSignup("communitylover", "community.lover", true),
			},
			additionalObjects: []client.Object{
				fake.NewSpace("communityspace", "member-1", "communitylover", space.WithTierName("appstudio")),
				fake.NewSpaceBinding("communitylover", toolchainv1alpha1.KubesawAuthenticatedUsername, "communityspace", "viewer"),
			},
			expectedWorkspaces: func(fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
				return []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "communityspace", "viewer", false, proxy.WithOwner("communitylover")),
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
			informerFunc := fake.GetInformerService(fakeClient)

			proxyMetrics := metrics.NewProxyMetrics(prometheus.NewRegistry())

			s := &handlers.SpaceLister{
				GetSignupFunc:          signupProvider,
				GetInformerServiceFunc: informerFunc,
				ProxyMetrics:           proxyMetrics,
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

func TestSpaceListerList(t *testing.T) {
	tt := map[string]struct {
		publicViewerEnabled bool
	}{
		"public-viewer is enabled":  {publicViewerEnabled: true},
		"public-viewer is disabled": {publicViewerEnabled: false},
	}

	for k, rtc := range tt {
		fakeSignupService, fakeClient := buildSpaceListerFakes(t)

		t.Run(k, func(t *testing.T) {
			// given
			tests := map[string]struct {
				username             string
				expectedWs           []toolchainv1alpha1.Workspace
				expectedErr          string
				expectedErrCode      int
				expectedWorkspace    string
				overrideSignupFunc   func(ctx *gin.Context, userID, username string, checkUserSignupComplete bool) (*signup.Signup, error)
				overrideInformerFunc func() service.InformerService
			}{
				"dancelover lists spaces": {
					username: "dance.lover",
					expectedWs: []toolchainv1alpha1.Workspace{
						workspaceFor(t, fakeClient, "dancelover", "admin", true),
						workspaceFor(t, fakeClient, "movielover", "other", false),
					},
					expectedErr: "",
				},
				"movielover lists spaces": {
					username: "movie.lover",
					expectedWs: []toolchainv1alpha1.Workspace{
						workspaceFor(t, fakeClient, "movielover", "admin", true),
					},
					expectedErr: "",
				},
				"signup has no compliant username set": {
					username:        "racing.lover",
					expectedWs:      []toolchainv1alpha1.Workspace{},
					expectedErr:     "",
					expectedErrCode: 200,
				},
				"space not initialized yet": {
					username:        "panda.lover",
					expectedWs:      []toolchainv1alpha1.Workspace{},
					expectedErr:     "",
					expectedErrCode: 200,
				},
				"no spaces found": {
					username:        "user.nospace",
					expectedWs:      []toolchainv1alpha1.Workspace{},
					expectedErr:     "",
					expectedErrCode: 200,
				},
				"informer error": {
					username:        "dance.lover",
					expectedWs:      []toolchainv1alpha1.Workspace{},
					expectedErr:     "list spacebindings error",
					expectedErrCode: 500,
					overrideInformerFunc: func() service.InformerService {
						inf := fake.NewFakeInformer()
						inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
							return nil, fmt.Errorf("list spacebindings error")
						}
						return inf
					},
				},
				"get signup error": {
					username:        "dance.lover",
					expectedWs:      []toolchainv1alpha1.Workspace{},
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
					signupProvider := fakeSignupService.GetSignupFromInformer
					if tc.overrideSignupFunc != nil {
						signupProvider = tc.overrideSignupFunc
					}

					informerFunc := fake.GetInformerService(fakeClient)
					if tc.overrideInformerFunc != nil {
						informerFunc = tc.overrideInformerFunc
					}

					proxyMetrics := metrics.NewProxyMetrics(prometheus.NewRegistry())

					s := &handlers.SpaceLister{
						GetSignupFunc:          signupProvider,
						GetInformerServiceFunc: informerFunc,
						ProxyMetrics:           proxyMetrics,
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
						require.Equal(t, len(tc.expectedWs), len(workspaceList.Items))
						for i := range tc.expectedWs {
							assert.Equal(t, tc.expectedWs[i].Name, workspaceList.Items[i].Name)
							assert.Equal(t, tc.expectedWs[i].Status, workspaceList.Items[i].Status)
						}
					}
				})
			}
		})
	}
}
