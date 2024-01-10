package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	spacetest "github.com/codeready-toolchain/toolchain-common/pkg/test/space"
)

func buildSpaceListerFakes(t *testing.T) (*fake.SignupService, *test.FakeClient) {
	fakeSignupService := fake.NewSignupService(
		newSignup("dancelover", "dance.lover", true),
		newSignup("movielover", "movie.lover", true),
		newSignup("pandalover", "panda.lover", true),
		newSignup("usernospace", "user.nospace", true),
		newSignup("foodlover", "food.lover", true),
		newSignup("animelover", "anime.lover", true),
		newSignup("carlover", "car.lover", true),
		newSignup("racinglover", "racing.lover", false),
		newSignup("parentspace", "parent.space", true),
		newSignup("childspace", "child.space", true),
		newSignup("grandchildspace", "grandchild.space", true),
	)

	// space that is not provisioned yet
	spaceNotProvisionedYet := fake.NewSpace("pandalover", "member-2", "pandalover")
	spaceNotProvisionedYet.Labels[toolchainv1alpha1.SpaceCreatorLabelKey] = ""

	// spacebinding associated with SpaceBindingRequest
	spaceBindingWithSBRonMovieLover := fake.NewSpaceBinding("foodlover-sb-from-sbr-on-movielover", "foodlover", "movielover", "maintainer")
	spaceBindingWithSBRonMovieLover.Labels[toolchainv1alpha1.SpaceBindingRequestLabelKey] = "foodlover-sbr"
	spaceBindingWithSBRonMovieLover.Labels[toolchainv1alpha1.SpaceBindingRequestNamespaceLabelKey] = "movielover-tenant"

	// spacebinding associated with SpaceBindingRequest on a dancelover,
	// which is also the parentSpace of foodlover
	spaceBindingWithSBRonDanceLover := fake.NewSpaceBinding("animelover-sb-from-sbr-on-dancelover", "animelover", "dancelover", "viewer")
	spaceBindingWithSBRonDanceLover.Labels[toolchainv1alpha1.SpaceBindingRequestLabelKey] = "animelover-sbr"
	spaceBindingWithSBRonDanceLover.Labels[toolchainv1alpha1.SpaceBindingRequestNamespaceLabelKey] = "dancelover-tenant"

	// spacebinding with SpaceBindingRequest but name is missing
	spaceBindingWithInvalidSBRName := fake.NewSpaceBinding("carlover-sb-from-sbr", "carlover", "animelover", "viewer")
	spaceBindingWithInvalidSBRName.Labels[toolchainv1alpha1.SpaceBindingRequestLabelKey] = "" // let's set the name to blank in order to trigger an error
	spaceBindingWithInvalidSBRName.Labels[toolchainv1alpha1.SpaceBindingRequestNamespaceLabelKey] = "anime-tenant"

	// spacebinding with SpaceBindingRequest but namespace is missing
	spaceBindingWithInvalidSBRNamespace := fake.NewSpaceBinding("animelover-sb-from-sbr", "animelover", "carlover", "viewer")
	spaceBindingWithInvalidSBRNamespace.Labels[toolchainv1alpha1.SpaceBindingRequestLabelKey] = "anime-sbr"
	spaceBindingWithInvalidSBRNamespace.Labels[toolchainv1alpha1.SpaceBindingRequestNamespaceLabelKey] = "" // let's set the name to blank in order to trigger an error

	fakeClient := fake.InitClient(t,
		// spaces
		fake.NewSpace("dancelover", "member-1", "dancelover"),
		fake.NewSpace("movielover", "member-1", "movielover"),
		fake.NewSpace("racinglover", "member-2", "racinglover"),
		fake.NewSpace("foodlover", "member-2", "foodlover", spacetest.WithSpecParentSpace("dancelover")),
		fake.NewSpace("animelover", "member-1", "animelover"),
		fake.NewSpace("carlover", "member-1", "carlover"),
		spaceNotProvisionedYet,
		fake.NewSpace("parentspace", "member-1", "parentspace"),
		fake.NewSpace("childspace", "member-1", "childspace", spacetest.WithSpecParentSpace("parentspace")),
		fake.NewSpace("grandchildspace", "member-1", "grandchildspace", spacetest.WithSpecParentSpace("childspace")),
		// noise space, user will have a different role here , just to make sure this is not returned anywhere in the tests
		fake.NewSpace("otherspace", "member-1", "otherspace", spacetest.WithSpecParentSpace("otherspace")),

		//spacebindings
		fake.NewSpaceBinding("dancer-sb1", "dancelover", "dancelover", "admin"),
		fake.NewSpaceBinding("dancer-sb2", "dancelover", "movielover", "other"),
		fake.NewSpaceBinding("moviegoer-sb", "movielover", "movielover", "admin"),
		fake.NewSpaceBinding("racer-sb", "racinglover", "racinglover", "admin"),
		fake.NewSpaceBinding("anime-sb", "animelover", "animelover", "admin"),
		fake.NewSpaceBinding("car-sb", "carlover", "carlover", "admin"),
		spaceBindingWithSBRonMovieLover,
		spaceBindingWithSBRonDanceLover,
		spaceBindingWithInvalidSBRName,
		spaceBindingWithInvalidSBRNamespace,
		fake.NewSpaceBinding("parent-sb1", "parentspace", "parentspace", "admin"),
		fake.NewSpaceBinding("child-sb1", "childspace", "childspace", "admin"),
		fake.NewSpaceBinding("grandchild-sb1", "grandchildspace", "grandchildspace", "admin"),
		// noise spacebinding, just to make sure this is not returned anywhere in the tests
		fake.NewSpaceBinding("parent-sb2", "parentspace", "otherspace", "contributor"),

		//nstemplatetier
		fake.NewBase1NSTemplateTier(),
	)

	return fakeSignupService, fakeClient
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
