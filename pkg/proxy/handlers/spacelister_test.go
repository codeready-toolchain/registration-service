package handlers_test

import (
	"context"
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

func TestHandleSpaceListRequest(t *testing.T) {

	//given
	fakeSignupService := fake.NewSignupService(
		newSignup("dancelover", "dance.lover", true),
		newSignup("movielover", "movie.lover", true),
		newSignup("pandalover", "panda.lover", true),
		newSignup("usernospace", "user.nospace", true),
		newSignup("racinglover", "racing.lover", false),
	)

	// space that is not provisioned yet
	spaceNotProvisionedYet := fake.NewSpace("member-2", "pandalover")
	spaceNotProvisionedYet.Labels[toolchainv1alpha1.SpaceCreatorLabelKey] = ""

	fakeClient := initFakeClient(t,
		// spaces
		fake.NewSpace("member-1", "dancelover"),
		fake.NewSpace("member-1", "movielover"),
		fake.NewSpace("member-2", "racinglover"),
		spaceNotProvisionedYet,

		//spacebindings
		fake.NewSpaceBinding("dancer-sb1", "dancelover", "dancelover", "admin"),
		fake.NewSpaceBinding("dancer-sb2", "dancelover", "movielover", "other"),
		fake.NewSpaceBinding("moviegoer-sb", "movielover", "movielover", "admin"),
		fake.NewSpaceBinding("racer-sb", "racinglover", "racinglover", "admin"),
	)

	s := &handlers.SpaceLister{
		GetSignupFunc:          fakeSignupService.GetSignupFromInformer,
		GetInformerServiceFunc: getFakeInformerService(t, fakeClient),
	}

	tests := map[string]struct {
		username          string
		expected          []toolchainv1alpha1.Workspace
		expectedErr       string
		expectedErrCode   int
		expectedWorkspace string
	}{
		"dancelover lists spaces": {
			username: "dance.lover",
			expected: []toolchainv1alpha1.Workspace{
				workspaceFor(t, fakeClient, "dancelover", "admin", true),
				workspaceFor(t, fakeClient, "movielover", "other", false),
			},
			expectedErr: "",
		},
		"movielover lists spaces": {
			username: "movie.lover",
			expected: []toolchainv1alpha1.Workspace{
				workspaceFor(t, fakeClient, "movielover", "admin", true),
			},
			expectedErr: "",
		},
		"dancelover gets dancelover space": {
			username: "dance.lover",
			expected: []toolchainv1alpha1.Workspace{
				workspaceFor(t, fakeClient, "dancelover", "admin", true),
			},
			expectedErr:       "",
			expectedWorkspace: "dancelover",
		},
		"dancelover gets movielover space": {
			username: "dance.lover",
			expected: []toolchainv1alpha1.Workspace{
				workspaceFor(t, fakeClient, "movielover", "other", false),
			},
			expectedErr:       "",
			expectedWorkspace: "movielover",
		},
		"movielover gets movielover space": {
			username: "movie.lover",
			expected: []toolchainv1alpha1.Workspace{
				workspaceFor(t, fakeClient, "movielover", "admin", true),
			},
			expectedErr:       "",
			expectedWorkspace: "movielover",
		},
		"movielover cannot get dancelover space": {
			username:          "movie.lover",
			expected:          []toolchainv1alpha1.Workspace{},
			expectedErr:       "\"workspaces.toolchain.dev.openshift.com \\\"dancelover\\\" not found\"",
			expectedWorkspace: "dancelover",
			expectedErrCode:   404,
		},
		"signup not ready yet": {
			username:        "racing.lover",
			expected:        []toolchainv1alpha1.Workspace{},
			expectedErr:     "user account not ready",
			expectedErrCode: 401,
		},
		"space not initialized yet": {
			username:        "panda.lover",
			expected:        []toolchainv1alpha1.Workspace{},
			expectedErr:     "",
			expectedErrCode: 200,
		},
		"no spaces found": {
			username:        "user.nospace",
			expected:        []toolchainv1alpha1.Workspace{},
			expectedErr:     "",
			expectedErrCode: 200,
		},
	}

	for k, tc := range tests {
		t.Run(k, func(t *testing.T) {
			// given
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)
			ctx.Set(rcontext.UsernameKey, tc.username)

			if tc.expectedWorkspace != "" {
				ctx.SetParamNames("workspace")
				ctx.SetParamValues(tc.expectedWorkspace)
			}

			// when
			err := s.HandleSpaceListRequest(ctx)

			// then
			if tc.expectedErr != "" {
				// error case
				require.Equal(t, tc.expectedErrCode, rec.Code)
				require.Contains(t, rec.Body.String(), tc.expectedErr)
			} else {
				require.NoError(t, err)
				if tc.expectedWorkspace != "" {
					// get workspace case
					workspace, decodeErr := decodeResponseToWorkspace(rec.Body.Bytes())
					require.NoError(t, decodeErr)
					require.Equal(t, 1, len(tc.expected), "test case should have exactly one expected item since it's a get request")
					for i := range tc.expected {
						assert.Equal(t, tc.expected[i].Name, workspace.Name)
						assert.Equal(t, tc.expected[i].Status, workspace.Status)
					}
				} else {
					// list workspace case
					workspaceList, decodeErr := decodeResponseToWorkspaceList(rec.Body.Bytes())
					require.NoError(t, decodeErr)
					require.Equal(t, len(tc.expected), len(workspaceList.Items))
					for i := range tc.expected {
						assert.Equal(t, tc.expected[i].Name, workspaceList.Items[i].Name)
						assert.Equal(t, tc.expected[i].Status, workspaceList.Items[i].Status)
					}
				}
			}
		})
	}
}

func newSignup(signupName, username string, ready bool) fake.SignupDef {
	return fake.Signup(signupName, &signup.Signup{
		Name:              signupName,
		CompliantUsername: signupName,
		Username:          username,
		Status: signup.Status{
			Ready: ready,
		},
	})
}

func getFakeInformerService(t *testing.T, fakeClient client.Client) func() service.InformerService {
	return func() service.InformerService {

		inf := fake.NewFakeInformer()
		inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
			space := &toolchainv1alpha1.Space{}
			err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: name}, space)
			return space, err
		}
		inf.ListSpaceBindingFunc = func(reqs ...labels.Requirement) ([]*toolchainv1alpha1.SpaceBinding, error) {
			labelMatch := client.MatchingLabels{}
			for _, r := range reqs {
				labelMatch[r.Key()] = r.Values().List()[0]
			}
			sbList := &toolchainv1alpha1.SpaceBindingList{}
			err := fakeClient.List(context.TODO(), sbList, labelMatch)
			sbs := []*toolchainv1alpha1.SpaceBinding{}
			for _, sb := range sbList.Items {
				sbs = append(sbs, sb.DeepCopy())
			}
			return sbs, err
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

func workspaceFor(t *testing.T, fakeClient client.Client, name, role string, isHomeWorkspace bool) toolchainv1alpha1.Workspace {
	// get the space for the user
	space := &toolchainv1alpha1.Space{}
	err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: configuration.Namespace()}, space)
	require.NoError(t, err)

	// create the workspace based on the space
	ws := commonproxy.NewWorkspace(name,
		commonproxy.WithObjectMetaFrom(space.ObjectMeta),
		commonproxy.WithNamespaces([]toolchainv1alpha1.SpaceNamespace{
			{
				Name: name + "-tenant",
				Type: "default",
			},
		}),
		commonproxy.WithOwner(name),
		commonproxy.WithRole(role),
	)
	// if the user is the same as the one who created the workspace, then expect type should be "home"
	if isHomeWorkspace {
		ws.Status.Type = "home"
	}
	return *ws
}
