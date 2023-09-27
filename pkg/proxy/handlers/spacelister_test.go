package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	rcontext "github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/gin-gonic/gin"
	"sigs.k8s.io/controller-runtime/pkg/client"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestSpaceLister(t *testing.T) {

	fakeSignupService := fake.NewSignupService(
		newSignup("dancelover", "dance.lover", true),
		newSignup("movielover", "movie.lover", true),
		newSignup("pandalover", "panda.lover", true),
		newSignup("usernospace", "user.nospace", true),
		newSignup("racinglover", "racing.lover", false),
	)

	// space that is not provisioned yet
	spaceNotProvisionedYet := fake.NewSpace("pandalover", "member-2", "pandalover")
	spaceNotProvisionedYet.Labels[toolchainv1alpha1.SpaceCreatorLabelKey] = ""

	fakeClient := initFakeClient(t,
		// spaces
		fake.NewSpace("dancelover", "member-1", "dancelover"),
		fake.NewSpace("movielover", "member-1", "movielover"),
		fake.NewSpace("racinglover", "member-2", "racinglover"),
		spaceNotProvisionedYet,

		//spacebindings
		fake.NewSpaceBinding("dancer-sb1", "dancelover", "dancelover", "admin"),
		fake.NewSpaceBinding("dancer-sb2", "dancelover", "movielover", "other"),
		fake.NewSpaceBinding("moviegoer-sb", "movielover", "movielover", "admin"),
		fake.NewSpaceBinding("racer-sb", "racinglover", "racinglover", "admin"),

		//nstemplatetier
		fake.NewBase1NSTemplateTier(),
	)

	t.Run("HandleSpaceListRequest", func(t *testing.T) {
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
					inf.ListSpaceBindingFunc = func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
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
				overrideSignupFunc: func(ctx *gin.Context, userID, username string, checkUserSignupComplete bool) (*signup.Signup, error) {
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

				informerFunc := getFakeInformerService(fakeClient)
				if tc.overrideInformerFunc != nil {
					informerFunc = tc.overrideInformerFunc
				}

				s := &handlers.SpaceLister{
					GetSignupFunc:          signupProvider,
					GetInformerServiceFunc: informerFunc,
				}

				e := echo.New()
				req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
				rec := httptest.NewRecorder()
				ctx := e.NewContext(req, rec)
				ctx.Set(rcontext.UsernameKey, tc.username)

				// when
				err := s.HandleSpaceListRequest(ctx)

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
						})),
				},
				expectedErr:       "",
				expectedWorkspace: "movielover",
			},
			"movielover gets movielover space": {
				username: "movie.lover",
				expectedWs: []toolchainv1alpha1.Workspace{
					workspaceFor(t, fakeClient, "movielover", "admin", true,
						commonproxy.WithAvailableRoles([]string{
							"admin", "viewer",
						})),
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
						})),
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
					informerFunc := getFakeInformerService(fakeClient, WithGetNSTemplateTierFunc(func(tierName string) (*toolchainv1alpha1.NSTemplateTier, error) {
						return nil, fmt.Errorf("nstemplatetier error")
					}))
					return informerFunc()
				},
				expectedWorkspace: "dancelover",
			},
			"too many spacebindings for user": {
				username:        "dance.lover",
				expectedWs:      []toolchainv1alpha1.Workspace{},
				expectedErr:     "Internal error occurred: expected only 1 spacebinding, got 2 for user dancelover and workspace dancelover",
				expectedErrCode: 500,
				overrideInformerFunc: func() service.InformerService {
					inf := fake.NewFakeInformer()
					inf.ListSpaceBindingFunc = func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
						// let's return more than 1 spacebinding to trigger the error
						return []toolchainv1alpha1.SpaceBinding{
							*fake.NewSpaceBinding("dancer-sb1", "dancelover", "dancelover", "admin"),
							*fake.NewSpaceBinding("dancer-sb2", "dancelover", "dancelover", "other"),
						}, nil
					}
					return inf
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
		}

		for k, tc := range tests {
			t.Run(k, func(t *testing.T) {
				// given
				signupProvider := fakeSignupService.GetSignupFromInformer
				if tc.overrideSignupFunc != nil {
					signupProvider = tc.overrideSignupFunc
				}

				informerFunc := getFakeInformerService(fakeClient)
				if tc.overrideInformerFunc != nil {
					informerFunc = tc.overrideInformerFunc
				}

				s := &handlers.SpaceLister{
					GetSignupFunc:          signupProvider,
					GetInformerServiceFunc: informerFunc,
				}

				e := echo.New()
				req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
				rec := httptest.NewRecorder()
				ctx := e.NewContext(req, rec)
				ctx.Set(rcontext.UsernameKey, tc.username)
				ctx.SetParamNames("workspace")
				ctx.SetParamValues(tc.expectedWorkspace)

				// when
				err := s.HandleSpaceGetRequest(ctx)

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

func newSignup(signupName, username string, ready bool) fake.SignupDef {
	compliantUsername := signupName
	if !ready {
		// signup is not ready, let's set compliant username to blank
		compliantUsername = ""
	}
	us := fake.Signup(signupName, &signup.Signup{
		Name:              signupName,
		Username:          username,
		CompliantUsername: compliantUsername,
		Status: signup.Status{
			Ready: ready,
		},
	})

	return us
}

type InformerServiceOptions func(informer *fake.Informer)

func WithGetNSTemplateTierFunc(getNsTemplateTierFunc func(tier string) (*toolchainv1alpha1.NSTemplateTier, error)) InformerServiceOptions {
	return func(informer *fake.Informer) {
		informer.GetNSTemplateTierFunc = getNsTemplateTierFunc
	}
}

func getFakeInformerService(fakeClient client.Client, options ...InformerServiceOptions) func() service.InformerService {
	return func() service.InformerService {

		inf := fake.NewFakeInformer()
		inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
			space := &toolchainv1alpha1.Space{}
			err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: name}, space)
			return space, err
		}
		inf.ListSpaceBindingFunc = func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			labelMatch := client.MatchingLabels{}
			for _, r := range reqs {
				labelMatch[r.Key()] = r.Values().List()[0]
			}
			sbList := &toolchainv1alpha1.SpaceBindingList{}
			err := fakeClient.List(context.TODO(), sbList, labelMatch)
			return sbList.Items, err
		}
		inf.GetNSTemplateTierFunc = func(tier string) (*toolchainv1alpha1.NSTemplateTier, error) {
			nsTemplateTier := &toolchainv1alpha1.NSTemplateTier{}
			err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: tier}, nsTemplateTier)
			return nsTemplateTier, err
		}

		for _, modify := range options {
			modify(&inf)
		}

		return inf
	}
}

func initFakeClient(t *testing.T, initObjs ...runtime.Object) *test.FakeClient {
	scheme := runtime.NewScheme()
	var AddToSchemes runtime.SchemeBuilder
	addToSchemes := append(AddToSchemes,
		toolchainv1alpha1.AddToScheme)
	err := addToSchemes.AddToScheme(scheme)
	require.NoError(t, err)
	cl := test.NewFakeClient(t, initObjs...)
	return cl
}

func decodeResponseToWorkspace(data []byte) (*toolchainv1alpha1.Workspace, error) {
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
	obj := &toolchainv1alpha1.Workspace{}
	err := runtime.DecodeInto(decoder, data, obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func decodeResponseToWorkspaceList(data []byte) (*toolchainv1alpha1.WorkspaceList, error) {
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
	obj := &toolchainv1alpha1.WorkspaceList{}
	err := runtime.DecodeInto(decoder, data, obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func workspaceFor(t *testing.T, fakeClient client.Client, name, role string, isHomeWorkspace bool, additionalWSOptions ...commonproxy.WorkspaceOption) toolchainv1alpha1.Workspace {
	// get the space for the user
	space := &toolchainv1alpha1.Space{}
	err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: configuration.Namespace()}, space)
	require.NoError(t, err)

	// create the workspace based on the space
	commonWSoptions := []commonproxy.WorkspaceOption{
		commonproxy.WithObjectMetaFrom(space.ObjectMeta),
		commonproxy.WithNamespaces([]toolchainv1alpha1.SpaceNamespace{
			{
				Name: "john-dev",
				Type: "default",
			},
			{
				Name: "john-stage",
			},
		}),
		commonproxy.WithOwner(name),
		commonproxy.WithRole(role),
	}
	ws := commonproxy.NewWorkspace(name,
		append(commonWSoptions, additionalWSOptions...)...,
	)
	// if the user is the same as the one who created the workspace, then expect type should be "home"
	if isHomeWorkspace {
		ws.Status.Type = "home"
	}
	return *ws
}
