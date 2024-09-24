package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	rcontext "github.com/codeready-toolchain/registration-service/pkg/context"
	infservice "github.com/codeready-toolchain/registration-service/pkg/informers/service"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/metrics"
	proxytest "github.com/codeready-toolchain/registration-service/pkg/proxy/test"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	commoncluster "github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	spacebindingrequesttest "github.com/codeready-toolchain/toolchain-common/pkg/test/spacebindingrequest"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSpaceListerGet(t *testing.T) {
	tt := map[string]struct {
		publicViewerEnabled bool
	}{
		"public-viewer enabled":  {publicViewerEnabled: true},
		"public-viewer disabled": {publicViewerEnabled: false},
	}

	for k, tc := range tt {
		t.Run(k, func(t *testing.T) {
			testSpaceListerGet(t, tc.publicViewerEnabled)
		})
	}
}

func testSpaceListerGet(t *testing.T, publicViewerEnabled bool) {

	memberFakeClient := test.NewFakeClient(t,
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

	memberClientErrorList := test.NewFakeClient(t)
	memberClientErrorList.MockList = func(ctx context.Context, list runtimeclient.ObjectList, opts ...runtimeclient.ListOption) error {
		if _, ok := list.(*toolchainv1alpha1.SpaceBindingRequestList); ok {
			return fmt.Errorf("mock list error")
		}
		return memberFakeClient.Client.List(ctx, list, opts...)
	}

	t.Run("HandleSpaceGetRequest", func(t *testing.T) {
		// given
		tests := map[string]struct {
			username               string
			expectedWs             func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace
			expectedErr            string
			expectedErrCode        int
			expectedWorkspace      string
			overrideSignupFunc     func(ctx *gin.Context, userID, username string, checkUserSignupComplete bool) (*signup.Signup, error)
			mockFakeClient         func(fakeClient *test.FakeClient)
			overrideGetMembersFunc func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster
			overrideMemberClient   *test.FakeClient
		}{
			"dancelover gets dancelover space": {
				username: "dance.lover",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "dancelover", "admin", true,
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
					)}
				},
				expectedErr:       "",
				expectedWorkspace: "dancelover",
			},
			"dancelover gets movielover space": {
				username: "dance.lover",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "movielover", "other", false,
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
					)}
				},
				expectedErr:       "",
				expectedWorkspace: "movielover",
			},
			"dancelover gets foodlover space": {
				username: "dance.lover",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "foodlover", "admin", false,
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
					)}
				},
				expectedErr:       "",
				expectedWorkspace: "foodlover",
			},
			"movielover gets movielover space": {
				username: "movie.lover",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "movielover", "admin", true,
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
					)}
				},
				expectedErr:       "",
				expectedWorkspace: "movielover",
			},
			"movielover cannot get dancelover space": {
				username:          "movie.lover",
				expectedWs:        nil,
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"dancelover\\\" not found\"",
				expectedWorkspace: "dancelover",
				expectedErrCode:   404,
			},
			"signup not ready yet": {
				username: "movie.lover",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "movielover", "admin", true,
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
					)}
				},
				expectedErr:       "",
				expectedWorkspace: "movielover",
			},
			"get nstemplatetier error": {
				username:        "dance.lover",
				expectedWs:      nil,
				expectedErr:     "nstemplatetier error",
				expectedErrCode: 500,
				mockFakeClient: func(fakeClient *test.FakeClient) {
					fakeClient.MockGet = func(ctx context.Context, key runtimeclient.ObjectKey, obj runtimeclient.Object, opts ...runtimeclient.GetOption) error {
						if key.Name == "base1ns" {
							return fmt.Errorf("nstemplatetier error")
						}
						return fakeClient.Client.Get(ctx, key, obj, opts...)
					}
				},
				expectedWorkspace: "dancelover",
			},
			"get signup error": {
				username:        "dance.lover",
				expectedWs:      nil,
				expectedErr:     "signup error",
				expectedErrCode: 500,
				overrideSignupFunc: func(_ *gin.Context, _, _ string, _ bool) (*signup.Signup, error) {
					return nil, fmt.Errorf("signup error")
				},
				expectedWorkspace: "dancelover",
			},
			"signup has no compliant username set": {
				username:          "racing.lover",
				expectedWs:        nil,
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"racinglover\\\" not found\"",
				expectedErrCode:   404,
				expectedWorkspace: "racinglover",
			},
			"list spacebindings error": {
				username:        "dance.lover",
				expectedWs:      nil,
				expectedErr:     "list spacebindings error",
				expectedErrCode: 500,
				mockFakeClient: func(fakeClient *test.FakeClient) {
					fakeClient.MockList = func(_ context.Context, _ runtimeclient.ObjectList, _ ...runtimeclient.ListOption) error {
						return fmt.Errorf("list spacebindings error")
					}
				},
				expectedWorkspace: "dancelover",
			},
			"unable to get space": {
				username:        "dance.lover",
				expectedWs:      nil,
				expectedErr:     "\"workspaces.toolchain.dev.openshift.com \\\"dancelover\\\" not found\"",
				expectedErrCode: 404,
				mockFakeClient: func(fakeClient *test.FakeClient) {
					fakeClient.MockGet = func(ctx context.Context, key runtimeclient.ObjectKey, obj runtimeclient.Object, opts ...runtimeclient.GetOption) error {
						if key.Name == "dancelover" {
							return fmt.Errorf("no space")
						}
						return fakeClient.Client.Get(ctx, key, obj, opts...)
					}
				},
				expectedWorkspace: "dancelover",
			},
			"unable to get parent-space": {
				username:        "food.lover",
				expectedWs:      nil,
				expectedErr:     "Internal error occurred: unable to get parent-space: parent-space error",
				expectedErrCode: 500,
				mockFakeClient: func(fakeClient *test.FakeClient) {
					fakeClient.MockGet = func(ctx context.Context, key runtimeclient.ObjectKey, obj runtimeclient.Object, opts ...runtimeclient.GetOption) error {
						if key.Name == "dancelover" {
							return fmt.Errorf("parent-space error")
						}
						return fakeClient.Client.Get(ctx, key, obj, opts...)
					}
				},
				expectedWorkspace: "foodlover",
			},
			"error spaceBinding request has no name": {
				username: "anime.lover",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "animelover", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
					)}
				},
				expectedErr:       "Internal error occurred: SpaceBindingRequest name not found on binding: carlover-sb-from-sbr",
				expectedErrCode:   500,
				expectedWorkspace: "animelover",
			},
			"error spaceBinding request has no namespace set": {
				username: "car.lover",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "carlover", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						}),
					)}
				},
				expectedErr:       "Internal error occurred: SpaceBindingRequest namespace not found on binding: animelover-sb-from-sbr",
				expectedErrCode:   500,
				expectedWorkspace: "carlover",
			},
			"parent can list parentspace": {
				username: "parent.space",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "parentspace", "admin", true,
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
					)}
				},
				expectedWorkspace: "parentspace",
			},
			"parent can list childspace": {
				username: "parent.space",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "childspace", "admin", false,
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
					)}
				},
				expectedWorkspace: "childspace",
			},
			"parent can list grandchildspace": {
				username: "parent.space",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "grandchildspace", "admin", false,
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
					)}
				},
				expectedWorkspace: "grandchildspace",
			},
			"child cannot list parentspace": {
				username:          "child.space",
				expectedWs:        nil,
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"parentspace\\\" not found\"",
				expectedErrCode:   404,
				expectedWorkspace: "parentspace",
			},
			"child can list childspace": {
				username:          "child.space",
				expectedWorkspace: "childspace",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "childspace", "admin", true,
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
					)}
				},
			},
			"child can list grandchildspace": {
				username:          "child.space",
				expectedWorkspace: "grandchildspace",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "grandchildspace", "admin", false,
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
					)}
				},
			},
			"grandchild can list grandchildspace": {
				username: "grandchild.space",
				expectedWs: func(t *testing.T, fakeClient *test.FakeClient) []toolchainv1alpha1.Workspace {
					return []toolchainv1alpha1.Workspace{workspaceFor(t, fakeClient, "grandchildspace", "admin", true,
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
					)}
				},
				expectedWorkspace: "grandchildspace",
			},
			"grandchild cannot list parentspace": {
				username:          "grandchild.space",
				expectedWorkspace: "parentspace",
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"parentspace\\\" not found\"",
				expectedErrCode:   404,
				expectedWs:        nil,
			},
			"grandchild cannot list childspace": {
				username:          "grandchild.space",
				expectedWorkspace: "childspace",
				expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"childspace\\\" not found\"",
				expectedErrCode:   404,
				expectedWs:        nil,
			},
			"no member clusters found": {
				username:          "movie.lover",
				expectedWorkspace: "movielover",
				expectedErr:       "no member clusters found",
				expectedErrCode:   500,
				overrideGetMembersFunc: func(_ ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
					return []*commoncluster.CachedToolchainCluster{}
				},
			},
			"error listing spacebinding requests": {
				username:             "movie.lover",
				expectedWorkspace:    "movielover",
				expectedErr:          "mock list error",
				expectedErrCode:      500,
				overrideMemberClient: memberClientErrorList,
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
				ctx.SetParamNames("workspace")
				ctx.SetParamValues(tc.expectedWorkspace)

				// when
				memberClient := memberFakeClient
				if tc.overrideMemberClient != nil {
					memberClient = tc.overrideMemberClient
				}
				getMembersFunc := proxytest.NewGetMembersFunc(memberClient)
				if tc.overrideGetMembersFunc != nil {
					getMembersFunc = tc.overrideGetMembersFunc
				}
				ctx.Set(rcontext.PublicViewerEnabled, publicViewerEnabled)
				err := handlers.HandleSpaceGetRequest(s, getMembersFunc)(ctx)

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
					var expectedWorkspaces []toolchainv1alpha1.Workspace
					if tc.expectedWs != nil {
						expectedWorkspaces = tc.expectedWs(t, fakeClient)
					}
					require.Len(t, expectedWorkspaces, 1, "test case should have exactly one expected item since it's a get request")
					for i := range expectedWorkspaces {
						assert.Equal(t, expectedWorkspaces[i].Name, workspace.Name)
						assert.Equal(t, expectedWorkspaces[i].Status, workspace.Status)
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

	tests := map[string]struct {
		username           string
		expectedErr        string
		workspaceRequest   string
		expectedWorkspace  func(t *testing.T, fakeClient *test.FakeClient) toolchainv1alpha1.Workspace
		mockFakeClient     func(fakeClient *test.FakeClient)
		overrideSignupFunc func(ctx *gin.Context, userID, username string, checkUserSignupComplete bool) (*signup.Signup, error)
	}{
		"get robin workspace": {
			username:         "robin.space",
			workspaceRequest: "robin",
			expectedWorkspace: func(t *testing.T, fakeClient *test.FakeClient) toolchainv1alpha1.Workspace {
				return workspaceFor(t, fakeClient, "robin", "admin", true)
			},
		},
		"invalid number of spacebindings": {
			username:         "batman.space",
			expectedErr:      "invalid number of SpaceBindings found for MUR:batman and Space:batman. Expected 1 got 2",
			workspaceRequest: "batman",
		},
		"user is unauthorized": {
			username:         "robin.space",
			workspaceRequest: "batman",
		},
		"usersignup not found": {
			username:          "invalid.user",
			workspaceRequest:  "batman",
			expectedWorkspace: nil, // user is not authorized
		},
		"space not found": {
			username:         "invalid.user",
			workspaceRequest: "batman",
			mockFakeClient: func(fakeClient *test.FakeClient) {
				fakeClient.MockGet = func(ctx context.Context, key runtimeclient.ObjectKey, obj runtimeclient.Object, opts ...runtimeclient.GetOption) error {
					if obj.GetObjectKind().GroupVersionKind().Kind == "Space" {
						return fmt.Errorf("no space")
					}
					return fakeClient.Client.Get(ctx, key, obj, opts...)
				}
			},
			expectedWorkspace: nil, // user is not authorized
		},
		"error getting usersignup": {
			username:         "invalid.user",
			workspaceRequest: "batman",
			mockFakeClient: func(fakeClient *test.FakeClient) {
				fakeClient.MockGet = func(ctx context.Context, key runtimeclient.ObjectKey, obj runtimeclient.Object, opts ...runtimeclient.GetOption) error {
					if obj.GetObjectKind().GroupVersionKind().Kind == "Space" {
						return fmt.Errorf("no space")
					}
					return fakeClient.Client.Get(ctx, key, obj, opts...)
				}
			},
			expectedWorkspace: nil, // user is not authorized
		},
		"get signup error": {
			username:         "batman.space",
			workspaceRequest: "batman",
			expectedErr:      "signup error",
			overrideSignupFunc: func(_ *gin.Context, _, _ string, _ bool) (*signup.Signup, error) {
				return nil, fmt.Errorf("signup error")
			},
			expectedWorkspace: nil,
		},
		"list spacebindings error": {
			username:         "robin.space",
			workspaceRequest: "robin",
			expectedErr:      "list spacebindings error",
			mockFakeClient: func(fakeClient *test.FakeClient) {
				fakeClient.MockList = func(_ context.Context, _ runtimeclient.ObjectList, _ ...runtimeclient.ListOption) error {
					return fmt.Errorf("list spacebindings error")
				}
			},
			expectedWorkspace: nil,
		},
		"kubesaw-authenticated can not get robin workspace": { // Because public viewer feature is NOT enabled
			username:          toolchainv1alpha1.KubesawAuthenticatedUsername,
			workspaceRequest:  "robin",
			expectedErr:       "",
			expectedWorkspace: nil,
		},
		"batman can not get robin workspace": { // Because public viewer feature is NOT enabled
			username:          "batman.space",
			workspaceRequest:  "robin",
			expectedErr:       "",
			expectedWorkspace: nil,
		},
	}

	for k, tc := range tests {
		t.Run(k, func(t *testing.T) {
			// given
			fakeClient := test.NewFakeClient(t,
				// space
				fake.NewSpace("batman", "member-1", "batman"),
				fake.NewSpace("robin", "member-1", "robin"),

				fake.NewSpaceBinding("robin-1", "robin", "robin", "admin"),
				// 2 spacebindings to force the error
				fake.NewSpaceBinding("batman-1", "batman", "batman", "admin"),
				fake.NewSpaceBinding("batman-2", "batman", "batman", "maintainer"),
				fake.NewSpaceBinding("community-robin", toolchainv1alpha1.KubesawAuthenticatedUsername, "robin", "viewer"),
			)
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

			if tc.expectedWorkspace != nil {
				require.Equal(t, tc.expectedWorkspace(t, fakeClient), *wrk)
			} else {
				require.Nil(t, wrk) // user is not authorized to get this workspace
			}
		})
	}
}

func TestSpaceListerGetPublicViewerEnabled(t *testing.T) {

	fakeSignupService := fake.NewSignupService(
		newSignup("batman", "batman.space", true),
		newSignup("robin", "robin.space", true),
		newSignup("gordon", "gordon.no-space", false),
	)

	fakeClient := test.NewFakeClient(t,
		// space
		fake.NewSpace("robin", "member-1", "robin"),
		fake.NewSpace("batman", "member-1", "batman"),

		// spacebindings
		fake.NewSpaceBinding("robin-1", "robin", "robin", "admin"),
		fake.NewSpaceBinding("batman-1", "batman", "batman", "admin"),

		fake.NewSpaceBinding("community-robin", toolchainv1alpha1.KubesawAuthenticatedUsername, "robin", "viewer"),
	)

	robinWS := workspaceFor(t, fakeClient, "robin", "admin", true)
	batmanWS := workspaceFor(t, fakeClient, "batman", "admin", true)
	publicRobinWS := func() *toolchainv1alpha1.Workspace {
		batmansRobinWS := robinWS.DeepCopy()
		batmansRobinWS.Status.Type = ""
		batmansRobinWS.Status.Role = "viewer"
		return batmansRobinWS
	}()
	tests := map[string]struct {
		username          string
		workspaceRequest  string
		expectedWorkspace *toolchainv1alpha1.Workspace
	}{
		"robin can get robin workspace": {
			username:          "robin.space",
			workspaceRequest:  "robin",
			expectedWorkspace: &robinWS,
		},
		"batman can get batman workspace": {
			username:          "batman.space",
			workspaceRequest:  "batman",
			expectedWorkspace: &batmanWS,
		},
		"batman can get robin workspace as public-viewer": {
			username:          "batman.space",
			workspaceRequest:  "robin",
			expectedWorkspace: publicRobinWS,
		},
		"robin can not get batman workspace": {
			username:          "robin.space",
			workspaceRequest:  "batman",
			expectedWorkspace: nil,
		},
		"gordon can get robin workspace as public-viewer": {
			username:          "gordon.no-space",
			workspaceRequest:  "robin",
			expectedWorkspace: publicRobinWS,
		},
		"gordon can not get batman workspace": {
			username:          "gordon.no-space",
			workspaceRequest:  "batman",
			expectedWorkspace: nil,
		},
	}

	for k, tc := range tests {

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
			ctx.SetParamNames("workspace")
			ctx.SetParamValues(tc.workspaceRequest)
			ctx.Set(rcontext.PublicViewerEnabled, true)

			// when
			wrk, err := handlers.GetUserWorkspace(ctx, s, tc.workspaceRequest)

			// then
			require.NoError(t, err)

			if tc.expectedWorkspace != nil {
				require.Equal(t, tc.expectedWorkspace, wrk)
			} else {
				require.Nil(t, wrk) // user is not authorized to get this workspace
			}
		})
	}
}

func TestGetUserWorkspaceWithBindingsWithPublicViewerEnabled(t *testing.T) {

	fakeSignupService := fake.NewSignupService(
		newSignup("batman", "batman.space", true),
		newSignup("robin", "robin.space", true),
		newSignup("gordon", "gordon.no-space", false),
	)

	fakeClient := test.NewFakeClient(t,
		// NSTemplateTiers
		fake.NewBase1NSTemplateTier(),

		// space
		fake.NewSpace("robin", "member-1", "robin"),
		fake.NewSpace("batman", "member-1", "batman"),

		// spacebindings
		fake.NewSpaceBinding("robin-1", "robin", "robin", "admin"),
		fake.NewSpaceBinding("batman-1", "batman", "batman", "admin"),

		fake.NewSpaceBinding("community-robin", toolchainv1alpha1.KubesawAuthenticatedUsername, "robin", "viewer"),
	)

	robinWS := workspaceFor(t, fakeClient, "robin", "admin", true,
		commonproxy.WithBindings([]toolchainv1alpha1.Binding{
			{
				MasterUserRecord: toolchainv1alpha1.KubesawAuthenticatedUsername,
				Role:             "viewer",
				AvailableActions: []string{},
			},
			{
				MasterUserRecord: "robin",
				Role:             "admin",
				AvailableActions: []string{},
			},
		}),
		commonproxy.WithAvailableRoles([]string{"admin", "viewer"}),
	)

	batmanWS := workspaceFor(t, fakeClient, "batman", "admin", true,
		commonproxy.WithBindings([]toolchainv1alpha1.Binding{
			{
				MasterUserRecord: "batman",
				Role:             "admin",
				AvailableActions: []string{},
			},
		}),
		commonproxy.WithAvailableRoles([]string{"admin", "viewer"}),
	)

	tests := map[string]struct {
		username          string
		workspaceRequest  string
		expectedWorkspace *toolchainv1alpha1.Workspace
	}{
		"robin can get robin workspace": {
			username:          "robin.space",
			workspaceRequest:  "robin",
			expectedWorkspace: &robinWS,
		},
		"batman can get batman workspace": {
			username:          "batman.space",
			workspaceRequest:  "batman",
			expectedWorkspace: &batmanWS,
		},
		"batman can get robin workspace": {
			username:         "batman.space",
			workspaceRequest: "robin",
			expectedWorkspace: func() *toolchainv1alpha1.Workspace {
				batmansRobinWS := robinWS.DeepCopy()
				batmansRobinWS.Status.Type = ""
				batmansRobinWS.Status.Role = "viewer"
				return batmansRobinWS
			}(),
		},
		"robin can not get batman workspace": {
			username:          "robin.space",
			workspaceRequest:  "batman",
			expectedWorkspace: nil,
		},
		"gordon can not get batman workspace": {
			username:          "gordon.no-space",
			workspaceRequest:  "batman",
			expectedWorkspace: nil,
		},
		"gordon can get robin workspace": {
			username:         "gordon.no-space",
			workspaceRequest: "robin",
			expectedWorkspace: func() *toolchainv1alpha1.Workspace {
				batmansRobinWS := robinWS.DeepCopy()
				batmansRobinWS.Status.Type = ""
				batmansRobinWS.Status.Role = "viewer"
				return batmansRobinWS
			}(),
		},
	}

	for k, tc := range tests {

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
			ctx.SetParamNames("workspace")
			ctx.SetParamValues(tc.workspaceRequest)
			ctx.Set(rcontext.PublicViewerEnabled, true)

			getMembersFuncMock := func(_ ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
				return []*commoncluster.CachedToolchainCluster{
					{
						Client: test.NewFakeClient(t),
						Config: &commoncluster.Config{
							Name: "not-me",
						},
					},
				}
			}

			// when
			wrk, err := handlers.GetUserWorkspaceWithBindings(ctx, s, tc.workspaceRequest, getMembersFuncMock)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedWorkspace, wrk)
		})
	}
}
