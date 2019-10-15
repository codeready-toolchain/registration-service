package signup_test

import (
	"errors"
	"testing"

	apiv1 "k8s.io/api/core/v1"
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
	svc, fakeClient := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	userSignup, err := svc.CreateUserSignup("jsmith", userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	gvk, err := apiutil.GVKForObject(userSignup, fakeClient.Scheme)
	require.NoError(s.T(), err)
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	values, err := fakeClient.Tracker.List(gvr, gvk, TestNamespace)
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

func (s *TestSignupServiceSuite) TestGetUserSignup() {
	svc, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	userSignup, err := svc.CreateUserSignup("jsmith", userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	/*
		TODO: more testing here once GetUserSignup() is completed
		retrieved, err := svc.GetUserSignup(userID.String())
		require.NoError(s.T(), err)
		require.NotNil(s.T(), retrieved)
	*/
}

func (s *TestSignupServiceSuite) TestUserSignupTransform() {
	svc, fakeClient := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	userSignup, err := svc.CreateUserSignup("jane.doe@redhat.com", userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	gvk, err := apiutil.GVKForObject(userSignup, fakeClient.Scheme)
	require.NoError(s.T(), err)
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	values, err := fakeClient.Tracker.List(gvr, gvk, TestNamespace)
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

	_, err = svc.CreateUserSignup("john#gmail.com", userID.String())
	require.Error(s.T(), err)
}

func (s *TestSignupServiceSuite) TestUserSignupNameExists() {
	svc, fakeClient := newSignupServiceWithFakeClient()
	err := fakeClient.Tracker.Add(&v1alpha1.UserSignup{
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
	require.NoError(s.T(), err)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	created, err := svc.CreateUserSignup("john@gmail.com", userID.String())
	require.NoError(s.T(), err)

	require.NotEqual(s.T(), "john-at-gmail-com", created.Name)
}

func (s *TestSignupServiceSuite) TestUserSignupCreateFails() {
	svc, fakeClient := newSignupServiceWithFakeClient()
	expectedErr := errors.New("an error occurred")
	fakeClient.MockCreate = func(*v1alpha1.UserSignup) (*v1alpha1.UserSignup, error) {
		return nil, expectedErr
	}

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	_, err = svc.CreateUserSignup("jack.smith@redhat.com", userID.String())
	require.Error(s.T(), err)
	require.Equal(s.T(), expectedErr, err)
}

func (s *TestSignupServiceSuite) TestUserSignupGetFails() {
	svc, fakeClient := newSignupServiceWithFakeClient()
	expectedErr := errors.New("an error occurred")
	fakeClient.MockGet = func(string) (*v1alpha1.UserSignup, error) {
		return nil, expectedErr
	}

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	_, err = svc.CreateUserSignup("hank.smith@redhat.com", userID.String())
	require.Error(s.T(), err)
	require.Equal(s.T(), expectedErr, err)
}

func (s *TestSignupServiceSuite) TestGetSignupNotFound() {
	svc, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	signup, err := svc.GetSignup(userID.String())
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusNotComplete() {
	svc, fakeClient := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			UserID:            userID.String(),
			Username:          "bill",
			CompliantUsername: "bill",
		},
		Status: v1alpha1.UserSignupStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:    v1alpha1.UserSignupComplete,
					Status:  apiv1.ConditionFalse,
					Reason:  "test_reason",
					Message: "test_message",
				},
			},
		},
	})
	require.NoError(s.T(), err)

	response, err := svc.GetSignup(userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), "bill", response.Username)
	require.False(s.T(), response.Status.Ready)
	require.Equal(s.T(), response.Status.Reason, "test_reason")
	require.Equal(s.T(), response.Status.Message, "test_message")
}

func (s *TestSignupServiceSuite) TestGetSignupNoStatusNotCompleteCondition() {
	svc, fakeClient := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			UserID:            userID.String(),
			Username:          "bill",
			CompliantUsername: "bill",
		},
		Status: v1alpha1.UserSignupStatus{},
	})
	require.NoError(s.T(), err)

	response, err := svc.GetSignup(userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), "bill", response.Username)
	require.False(s.T(), response.Status.Ready)
	require.Equal(s.T(), response.Status.Reason, signup.SignupReasonNoCondition)
	require.Equal(s.T(), response.Status.Message, signup.SignupMessageNoCondition)
}

func newSignupServiceWithFakeClient() (signup.Service, *fake.FakeUserSignupClient) {
	fakeClient := fake.NewFakeUserSignupClient(TestNamespace)
	fakeMURClient := fake.NewFakeMasterUserRecordClient(TestNamespace)
	return &signup.ServiceImpl{
		Namespace:         TestNamespace,
		UserSignups:       fakeClient,
		MasterUserRecords: fakeMURClient,
	}, fakeClient
}
