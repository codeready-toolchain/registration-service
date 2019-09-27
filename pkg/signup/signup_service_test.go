package signup_test

import (
	"context"
	"errors"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	TestNamespace = "test-namespace-123"
)

type TestSignupServiceSuite struct {
	testutils.UnitTestSuite
}

func TestRunSignupServiceSuite(t *testing.T) {
	suite.Run(t, &TestSignupServiceSuite{testutils.UnitTestSuite{}})
}

func (s *TestSignupServiceSuite) TestNewSignupService() {
	// Simply test creation of the service, which should fail as the kubernetes env variables are not set
	_, err := signup.NewSignupService(configuration.CreateEmptyRegistry())
	require.Error(s.T(), err)
}

func (s *TestSignupServiceSuite) TestCreateUserSignup() {
	svc, fake := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	userSignup, err := svc.CreateUserSignup(context.Background(), "jsmith", userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	gvk, err := apiutil.GVKForObject(userSignup, fake.Scheme)
	require.NoError(s.T(), err)
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	values, err := fake.Tracker.List(gvr, gvk, TestNamespace)
	require.NoError(s.T(), err)

	userSignups := values.(*v1alpha1.UserSignupList)
	require.NotEmpty(s.T(), userSignups.Items)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.Equal(s.T(), "jsmith", val.Name)
	require.Equal(s.T(), TestNamespace, val.Namespace)
	require.Equal(s.T(), userID.String(), val.Spec.UserID)
	require.Equal(s.T(), "jsmith", val.Spec.Username)
	require.False(s.T(), val.Spec.Approved)
}

func (s *TestSignupServiceSuite) TestUserSignupTransform() {
	svc, fake := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	userSignup, err := svc.CreateUserSignup(context.Background(), "jane.doe@redhat.com", userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	gvk, err := apiutil.GVKForObject(userSignup, fake.Scheme)
	require.NoError(s.T(), err)
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	values, err := fake.Tracker.List(gvr, gvk, TestNamespace)
	require.NoError(s.T(), err)

	userSignups := values.(*v1alpha1.UserSignupList)
	require.NotEmpty(s.T(), userSignups.Items)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.Equal(s.T(), "jane-doe-at-redhat-com", val.Name)
	require.Equal(s.T(), userID.String(), val.Spec.UserID)
}

func (s *TestSignupServiceSuite) TestUserSignupInvalidName() {
	svc, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	_, err = svc.CreateUserSignup(context.Background(), "john#gmail.com", userID.String())
	require.Error(s.T(), err)
}

func (s *TestSignupServiceSuite) TestUserSignupNameExists() {
	svc, fake := newSignupServiceWithFakeClient()
	fake.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "john-at-gmail-com",
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			UserID: "foo",
		},
		Status: v1alpha1.UserSignupStatus{},
	})

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	created, err := svc.CreateUserSignup(context.Background(), "john@gmail.com", userID.String())
	require.NoError(s.T(), err)

	require.NotEqual(s.T(), "john-at-gmail-com", created.Name)
}

func (s *TestSignupServiceSuite) TestUserSignupCreateFails() {
	svc, fake := newSignupServiceWithFakeClient()
	fake.MockCreate = func(*v1alpha1.UserSignup) (*v1alpha1.UserSignup, error) {
		return nil, errors.New("an error occurred")
	}

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	_, err = svc.CreateUserSignup(context.Background(), "jack.smith@redhat.com", userID.String())
	require.Error(s.T(), err)
}

func (s *TestSignupServiceSuite) TestUserSignupGetFails() {
	svc, fake := newSignupServiceWithFakeClient()
	expectedErr := errors.New("an error occurred")
	fake.MockGet = func(string) (*v1alpha1.UserSignup, error) {
		return nil, expectedErr
	}

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	_, err = svc.CreateUserSignup(context.Background(), "hank.smith@redhat.com", userID.String())
	require.Error(s.T(), err)
	require.Equal(s.T(), expectedErr, err)
}

func newSignupServiceWithFakeClient() (signup.SignupService, *fake.FakeUserSignupClient) {
	fake := fake.NewFakeUserSignupClient(TestNamespace)
	return &signup.SignupServiceImpl{
		Namespace:   TestNamespace,
		UserSignups: fake,
	}, fake
}
