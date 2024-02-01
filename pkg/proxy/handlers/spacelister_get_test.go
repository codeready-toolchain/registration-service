package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	proxytest "github.com/codeready-toolchain/registration-service/pkg/proxy/test"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	rcontext "github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	spacetest "github.com/codeready-toolchain/toolchain-common/pkg/test/space"
	spacebindingrequesttest "github.com/codeready-toolchain/toolchain-common/pkg/test/spacebindingrequest"
)

func TestSpaceListerGet(t *testing.T) {
	fakeSignupService, fakeClient := buildSpaceListerFakes(t)

	memberFakeClient := fake.InitClient(t,
		// spacebinding requests
		spacebindingrequesttest.NewSpaceBindingRequest("animelover-sbr", "dancelover-dev",
			spacebindingrequesttest.WithSpaceRole("viewer"),
			spacebindingrequesttest.WithMUR("animelover"),
			spacebindingrequesttest.WithCondition(spacebindingrequesttest.Ready()),
		),
		spacebindingrequesttest.NewSpaceBindingRequest("failing-sbr", "dancelover-dev",
			spacebindingrequesttest.WithSpaceRole("admin"),
			spacebindingrequesttest.WithMUR("someuser"),
			spacebindingrequesttest.WithCondition(spacebindingrequesttest.UnableToCreateSpaceBinding("unable to find user 'someuser'")),
		),
	)

	t.Run("HandleSpaceGetRequest", func(t *testing.T) {
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
			"dancelover gets dancelover space": {
				username: "dance.lover",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "dancelover", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						},
						),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "animelover",
								Role:             "viewer",
								AvailableActions: []string{"update", "delete"},
								BindingRequest: &toolchainv1alpha1.BindingRequest{ // animelover was granted access to dancelover workspace using SpaceBindingRequest
									Name:      "animelover-sbr",
									Namespace: "dancelover-dev",
								},
							},
							{
								MasterUserRecord: "dancelover",
								Role:             "admin",
								AvailableActions: []string(nil), // this is system generated so no actions for the user
							},
							{
								MasterUserRecord: "someuser",
								Role:             "admin",
								AvailableActions: []string{"update", "delete"},
								BindingRequest: &toolchainv1alpha1.BindingRequest{ // this SBR should show in the list even if it doesn't have a SpaceBinding associated
									Name:      "failing-sbr",
									Namespace: "dancelover-dev",
								},
							},
						}),
					),
				},
				expectedErr:       "",
				expectedWorkspace: "dancelover",
			},
			"dancelover gets movielover space": {
				username: "dance.lover",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "movielover", "other", false,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "dancelover",
								Role:             "other",
								AvailableActions: []string(nil), // this is system generated so no actions for the user
							},
							{
								MasterUserRecord: "foodlover",
								Role:             "maintainer",
								AvailableActions: []string{"update", "delete"},
								BindingRequest: &toolchainv1alpha1.BindingRequest{ // foodlover was granted access to movielover workspace using SpaceBindingRequest
									Name:      "foodlover-sbr",
									Namespace: "movielover-dev",
								},
							},
							{
								MasterUserRecord: "movielover",
								Role:             "admin",
								AvailableActions: []string(nil), // this is system generated so no actions for the user
							},
						}),
					),
				},
				expectedErr:       "",
				expectedWorkspace: "movielover",
			},
			"dancelover gets foodlover space": {
				username: "dance.lover",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "foodlover", "admin", false,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "animelover",
								Role:             "viewer",             // animelover was granted access via SBR , but on the parentSpace,
								AvailableActions: []string{"override"}, // since the binding is inherited from parent space, then it can only be overridden
							},
							{
								MasterUserRecord: "dancelover",
								Role:             "admin",              // dancelover is admin since it's admin on the parent space,
								AvailableActions: []string{"override"}, // since the binding is inherited from parent space, then it can only be overridden
							},
						}),
					),
				},
				expectedErr:       "",
				expectedWorkspace: "foodlover",
			},
			"movielover gets movielover space": {
				username: "movie.lover",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "movielover", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						// bindings are in alphabetical order using the MUR name
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "dancelover",
								Role:             "other",
								AvailableActions: []string(nil), // this is system generated so no actions for the user
							},
							{
								MasterUserRecord: "foodlover",
								Role:             "maintainer",
								AvailableActions: []string{"update", "delete"},
								BindingRequest: &toolchainv1alpha1.BindingRequest{
									Name:      "foodlover-sbr",
									Namespace: "movielover-dev",
								},
							},
							{
								MasterUserRecord: "movielover",
								Role:             "admin",
								AvailableActions: []string(nil), // this is system generated so no actions for the user
							},
						}),
					),
				},
				expectedErr:       "",
				expectedWorkspace: "movielover",
			},
			"movielover cannot get dancelover space": {
				username:          "movie.lover",
				expectedWs:        []toolchainv1alpha1.Workspace{},
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"dancelover\\\" not found\"",
				expectedWorkspace: "dancelover",
				expectedErrCode:   404,
			},
			"signup not ready yet": {
				username: "movie.lover",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "movielover", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "dancelover",
								Role:             "other",
								AvailableActions: []string(nil), // this is system generated so no actions for the user
							},
							{
								MasterUserRecord: "foodlover", // foodlover was granted access to movielover workspace using SpaceBindingRequest
								Role:             "maintainer",
								AvailableActions: []string{"update", "delete"},
								BindingRequest: &toolchainv1alpha1.BindingRequest{
									Name:      "foodlover-sbr",
									Namespace: "movielover-dev",
								},
							},
							{
								MasterUserRecord: "movielover",
								Role:             "admin",
								AvailableActions: []string(nil), // this is system generated so no actions for the user
							},
						}),
					),
				},
				expectedErr:       "",
				expectedWorkspace: "movielover",
			},
			"get nstemplatetier error": {
				username:        "dance.lover",
				expectedWs:      []toolchainv1alpha1.Workspace{},
				expectedErr:     "nstemplatetier error",
				expectedErrCode: 500,
				overrideInformerFunc: func() service.InformerService {
					informerFunc := fake.GetInformerService(fakeClient, fake.WithGetNSTemplateTierFunc(func(tierName string) (*toolchainv1alpha1.NSTemplateTier, error) {
						return nil, fmt.Errorf("nstemplatetier error")
					}))
					return informerFunc()
				},
				expectedWorkspace: "dancelover",
			},
			"get signup error": {
				username:        "dance.lover",
				expectedWs:      []toolchainv1alpha1.Workspace{},
				expectedErr:     "signup error",
				expectedErrCode: 500,
				overrideSignupFunc: func(ctx *gin.Context, userID, username string, checkUserSignupComplete bool) (*signup.Signup, error) {
					return nil, fmt.Errorf("signup error")
				},
				expectedWorkspace: "dancelover",
			},
			"signup has no compliant username set": {
				username:          "racing.lover",
				expectedWs:        []toolchainv1alpha1.Workspace{},
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"racinglover\\\" not found\"",
				expectedErrCode:   404,
				expectedWorkspace: "racinglover",
			},
			"list spacebindings error": {
				username:        "dance.lover",
				expectedWs:      []toolchainv1alpha1.Workspace{},
				expectedErr:     "list spacebindings error",
				expectedErrCode: 500,
				overrideInformerFunc: func() service.InformerService {
					listSpaceBindingFunc := func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
						return nil, fmt.Errorf("list spacebindings error")
					}
					return fake.GetInformerService(fakeClient, fake.WithListSpaceBindingFunc(listSpaceBindingFunc))()
				},
				expectedWorkspace: "dancelover",
			},
			"unable to get space": {
				username:        "dance.lover",
				expectedWs:      []toolchainv1alpha1.Workspace{},
				expectedErr:     "\"workspaces.toolchain.dev.openshift.com \\\"dancelover\\\" not found\"",
				expectedErrCode: 404,
				overrideInformerFunc: func() service.InformerService {
					getSpaceFunc := func(name string) (*toolchainv1alpha1.Space, error) {
						return nil, fmt.Errorf("no space")
					}
					return fake.GetInformerService(fakeClient, fake.WithGetSpaceFunc(getSpaceFunc))()
				},
				expectedWorkspace: "dancelover",
			},
			"unable to get parent-space": {
				username:        "food.lover",
				expectedWs:      []toolchainv1alpha1.Workspace{},
				expectedErr:     "Internal error occurred: unable to get parent-space: parent-space error",
				expectedErrCode: 500,
				overrideInformerFunc: func() service.InformerService {
					getSpaceFunc := func(name string) (*toolchainv1alpha1.Space, error) {
						if name == "dancelover" {
							// return the error only when trying to get the parent space
							return nil, fmt.Errorf("parent-space error")
						}
						// return the foodlover space
						return fake.NewSpace("foodlover", "member-2", "foodlover", spacetest.WithSpecParentSpace("dancelover")), nil
					}
					return fake.GetInformerService(fakeClient, fake.WithGetSpaceFunc(getSpaceFunc))()
				},
				expectedWorkspace: "foodlover",
			},
			"error spaceBinding request has no name": {
				username: "anime.lover",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "animelover", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
					),
				},
				expectedErr:       "Internal error occurred: SpaceBindingRequest name not found on binding: carlover-sb-from-sbr",
				expectedErrCode:   500,
				expectedWorkspace: "animelover",
			},
			"error spaceBinding request has no namespace set": {
				username: "car.lover",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "carlover", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
					),
				},
				expectedErr:       "Internal error occurred: SpaceBindingRequest namespace not found on binding: animelover-sb-from-sbr",
				expectedErrCode:   500,
				expectedWorkspace: "carlover",
			},
			"parent can list parentspace": {
				username: "parent.space",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "parentspace", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "parentspace",
								Role:             "admin",
								AvailableActions: []string(nil), // this is system generated so no actions for the user
							},
						}),
					),
				},
				expectedWorkspace: "parentspace",
			},
			"parent can list childspace": {
				username: "parent.space",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "childspace", "admin", false,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "childspace",
								Role:             "admin",
								AvailableActions: []string(nil),
							},
							{
								MasterUserRecord: "parentspace",
								Role:             "admin",
								AvailableActions: []string{"override"},
							},
						}),
					),
				},
				expectedWorkspace: "childspace",
			},
			"parent can list grandchildspace": {
				username: "parent.space",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "grandchildspace", "admin", false,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "childspace",
								Role:             "admin",
								AvailableActions: []string{"override"},
							},
							{
								MasterUserRecord: "grandchildspace",
								Role:             "admin",
								AvailableActions: []string(nil),
							},
							{
								MasterUserRecord: "parentspace",
								Role:             "admin",
								AvailableActions: []string{"override"},
							},
						}),
					),
				},
				expectedWorkspace: "grandchildspace",
			},
			"child cannot list parentspace": {
				username:          "child.space",
				expectedWs:        []toolchainv1alpha1.Workspace{},
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"parentspace\\\" not found\"",
				expectedErrCode:   404,
				expectedWorkspace: "parentspace",
			},
			"child can list childspace": {
				username:          "child.space",
				expectedWorkspace: "childspace",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "childspace", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "childspace",
								Role:             "admin",
								AvailableActions: []string(nil),
							},
							{
								MasterUserRecord: "parentspace",
								Role:             "admin",
								AvailableActions: []string{"override"},
							},
						}),
					),
				},
			},
			"child can list grandchildspace": {
				username:          "child.space",
				expectedWorkspace: "grandchildspace",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "grandchildspace", "admin", false,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "childspace",
								Role:             "admin",
								AvailableActions: []string{"override"},
							},
							{
								MasterUserRecord: "grandchildspace",
								Role:             "admin",
								AvailableActions: []string(nil),
							},
							{
								MasterUserRecord: "parentspace",
								Role:             "admin",
								AvailableActions: []string{"override"},
							},
						}),
					),
				},
			},
			"grandchild can list grandchildspace": {
				username: "grandchild.space",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "grandchildspace", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
						commonproxy.WithBindings([]toolchainv1alpha1.Binding{
							{
								MasterUserRecord: "childspace",
								Role:             "admin",
								AvailableActions: []string{"override"},
							},
							{
								MasterUserRecord: "grandchildspace",
								Role:             "admin",
								AvailableActions: []string(nil),
							},
							{
								MasterUserRecord: "parentspace",
								Role:             "admin",
								AvailableActions: []string{"override"},
							},
						}),
					),
				},
				expectedWorkspace: "grandchildspace",
			},
			"grandchild cannot list parentspace": {
				username:          "grandchild.space",
				expectedWorkspace: "parentspace",
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"parentspace\\\" not found\"",
				expectedErrCode:   404,
				expectedWs:        []toolchainv1alpha1.Workspace{},
			},
			"grandchild cannot list childspace": {
				username:          "grandchild.space",
				expectedWorkspace: "childspace",
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"childspace\\\" not found\"",
				expectedErrCode:   404,
				expectedWs:        []toolchainv1alpha1.Workspace{},
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
				ctx.SetParamNames("workspace")
				ctx.SetParamValues(tc.expectedWorkspace)

				// when
				err := handlers.HandleSpaceGetRequest(s, proxytest.NewGetMembersFunc(memberFakeClient))(ctx)

				// then
				if tc.expectedErr != "" {
					// error case
					require.Equal(t, tc.expectedErrCode, rec.Code)
					require.Contains(t, rec.Body.String(), tc.expectedErr)
				} else {
					require.NoError(t, err)
					// get workspace case
					workspace, decodeErr := decodeResponseToWorkspace(rec.Body.Bytes())
					require.NoError(t, decodeErr)
					require.Equal(t, 1, len(tc.expectedWs), "test case should have exactly one expected item since it's a get request")
					for i := range tc.expectedWs {
						assert.Equal(t, tc.expectedWs[i].Name, workspace.Name)
						assert.Equal(t, tc.expectedWs[i].Status, workspace.Status)
					}
				}
			})
		}
	})
}

func TestGetUserWorkspace(t *testing.T) {

	fakeSignupService := fake.NewSignupService(
		newSignup("batman", "batman.space", true),
		newSignup("robin", "robin.space", true),
	)

	fakeClient := fake.InitClient(t,
		// space
		fake.NewSpace("batman", "member-1", "batman"),

		// 2 spacebindings to force the error
		fake.NewSpaceBinding("batman-1", "batman", "batman", "admin"),
		fake.NewSpaceBinding("batman-2", "batman", "batman", "maintainer"),
	)

	tests := map[string]struct {
		username          string
		expectedErr       string
		workspaceRequest  string
		expectedWorkspace string
	}{
		"invalid number of spacebindings": {
			username:         "batman.space",
			expectedErr:      "invalid number of SpaceBindings found for MUR:batman and Space:batman. Expected 1 got 2",
			workspaceRequest: "batman",
		},
		"user is unauthorized": {
			username:         "robin.space",
			workspaceRequest: "batman",
		},
	}

	for k, tc := range tests {
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
			ctx.SetParamNames("workspace")
			ctx.SetParamValues(tc.workspaceRequest)

			// when
			wrk, err := handlers.GetUserWorkspace(ctx, s, tc.workspaceRequest)

			// then
			if tc.expectedErr != "" {
				// error case
				require.Error(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			if tc.expectedWorkspace != "" {
				require.Equal(t, wrk, tc.expectedWorkspace)
			} else {
				require.Nil(t, wrk) // user is not authorized to get this workspace
			}
		})
	}
}
