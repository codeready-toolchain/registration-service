package signup_test

import (
	"context"
	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"testing"
)

const (
	TestNamespace = "test-namespace-123"
)

func TestCreateUserSignup(t *testing.T) {
	svc, fake := newSignupServiceWithFakeClient()

	userSignup, err := svc.CreateUserSignup(context.Background(), "jsmith", "12345ABCDE")
	require.NoError(t, err)

	require.NotNil(t, userSignup)

	require.NoError(t, err)

	gvk, err := apiutil.GVKForObject(userSignup, fake.Scheme)
	require.NoError(t, err)
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	values, err := fake.Tracker.List(gvr, gvk, TestNamespace)
	require.NoError(t, err)

	userSignups := values.(*v1alpha1.UserSignupList)
	require.NotEmpty(t, userSignups.Items)
	require.Len(t, userSignups.Items, 1)
}

func newSignupServiceWithFakeClient() (signup.SignupService, *fake.FakeUserSignupClient) {
	fake := fake.NewFakeUserSignupClient(TestNamespace)
	return &signup.SignupServiceImpl{
		Namespace: TestNamespace,
		Client:    fake,
	}, fake
}
