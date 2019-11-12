package signup_test

import (
	"errors"
	"fmt"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	TestNamespace = "test-namespace-123"
)

type TestSignupServiceSuite struct {
	test.UnitTestSuite
}

func TestRunSignupServiceSuite(t *testing.T) {
	suite.Run(t, &TestSignupServiceSuite{test.UnitTestSuite{}})
}

func (s *TestSignupServiceSuite) TestCreateUserSignup() {
	svc, fakeClient, _ := newSignupServiceWithFakeClient()

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
	require.Equal(s.T(), TestNamespace, val.Namespace)
	require.Equal(s.T(), userID.String(), val.Name)
	require.Equal(s.T(), "jsmith", val.Spec.Username)
	require.Equal(s.T(), "jsmith", val.Spec.CompliantUsername)
	require.False(s.T(), val.Spec.Approved)
}

func (s *TestSignupServiceSuite) TestUserSignupTransform() {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	check := func(userSignupToBeReturnedByClient *v1alpha1.UserSignup, errToBeReturnedByClient error) {
		svc, userSignupsClient, _ := newSignupServiceWithFakeClient()

		userSignupsClient.MockGet = func(s string) (*v1alpha1.UserSignup, error) {
			return userSignupToBeReturnedByClient, errToBeReturnedByClient
		}
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)

		userSignup, err := svc.CreateUserSignup("jane.doe@redhat.com", userID.String())
		require.NoError(s.T(), err)
		require.NotNil(s.T(), userSignup)

		gvk, err := apiutil.GVKForObject(userSignup, userSignupsClient.Scheme)
		require.NoError(s.T(), err)
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)

		values, err := userSignupsClient.Tracker.List(gvr, gvk, TestNamespace)
		require.NoError(s.T(), err)

		userSignups := values.(*v1alpha1.UserSignupList)
		require.NotEmpty(s.T(), userSignups.Items)
		require.Len(s.T(), userSignups.Items, 1)

		val := userSignups.Items[0]
		require.Equal(s.T(), "jane-doe-at-redhat-com", val.Spec.CompliantUsername)
		require.Equal(s.T(), userID.String(), val.Name)
	}

	s.Run("UserSignup not found and client returns nil", func() {
		check(nil, kubeerr.NewNotFound(v1alpha1.SchemeGroupVersion.WithResource(userID.String()).GroupResource(), userID.String()))
	})

	s.Run("UserSignup not found and client returns empty UserSignup", func() {
		check(&v1alpha1.UserSignup{}, kubeerr.NewNotFound(v1alpha1.SchemeGroupVersion.WithResource(userID.String()).GroupResource(), userID.String()))
	})

	s.Run("unable to transform after N attempts", func() {
		svc, userSignupsClient, _ := newSignupServiceWithFakeClient()
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)
		userSignupsClient.MockGet = func(s string) (*v1alpha1.UserSignup, error) {
			return &v1alpha1.UserSignup{}, nil // Always return some UserSignup
		}

		_, err = svc.CreateUserSignup("jane.doe@redhat.com", userID.String())
		require.EqualError(s.T(), err, "unable to transform username [jane.doe@redhat.com] even after 1000 attempts")
	})
}

func (s *TestSignupServiceSuite) TestUserSignupInvalidName() {
	svc, _, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	_, err = svc.CreateUserSignup("john#gmail.com", userID.String())
	require.EqualError(s.T(), err, "transformed username [john#gmail.com] is invalid")
}

func (s *TestSignupServiceSuite) TestUserSignupNameExists() {
	svc, fakeClient, _ := newSignupServiceWithFakeClient()
	err := fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "john-at-gmail-com",
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			Username: "john@gmail.com",
		},
		Status: v1alpha1.UserSignupStatus{},
	})
	require.NoError(s.T(), err)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	created, err := svc.CreateUserSignup("john@gmail.com", userID.String())
	require.NoError(s.T(), err)

	require.Equal(s.T(), "john-at-gmail-com-1", created.Spec.CompliantUsername)
}

func (s *TestSignupServiceSuite) TestUserSignupCreateFails() {
	svc, fakeClient, _ := newSignupServiceWithFakeClient()
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
	svc, fakeClient, _ := newSignupServiceWithFakeClient()
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
	svc, _, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	signup, err := svc.GetSignup(userID.String())
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)
}

func (s *TestSignupServiceSuite) TestGetSignupGetFails() {
	svc, fakeClient, _ := newSignupServiceWithFakeClient()
	expectedErr := errors.New("an error occurred")
	fakeClient.MockGet = func(string) (*v1alpha1.UserSignup, error) {
		return nil, expectedErr
	}

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	_, err = svc.GetSignup(userID.String())
	require.Error(s.T(), err)
	require.Equal(s.T(), expectedErr, err)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusNotComplete() {
	svc, fakeClient, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
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
	svc, fakeClient, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
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
	require.Equal(s.T(), "PendingApproval", response.Status.Reason)
	require.Equal(s.T(), "", response.Status.Message)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusOK() {
	svc, fakeClient, fakeMURClient := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:          "ted",
			CompliantUsername: "ted",
		},
		Status: v1alpha1.UserSignupStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:   v1alpha1.UserSignupComplete,
					Status: apiv1.ConditionTrue,
				},
			},
		},
	})
	require.NoError(s.T(), err)

	err = fakeMURClient.Tracker.Add(&v1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ted",
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.MasterUserRecordSpec{
			UserID:        "",
			Disabled:      false,
			Deprovisioned: false,
			UserAccounts:  []v1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
		},
		Status: v1alpha1.MasterUserRecordStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:    v1alpha1.MasterUserRecordReady,
					Status:  apiv1.ConditionTrue,
					Reason:  "mur_ready_reason",
					Message: "mur_ready_message",
				},
			},
			UserAccounts: nil,
		},
	})
	require.NoError(s.T(), err)

	response, err := svc.GetSignup(userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), "ted", response.Username)
	assert.True(s.T(), response.Status.Ready)
	assert.Equal(s.T(), response.Status.Reason, "mur_ready_reason")
	assert.Equal(s.T(), response.Status.Message, "mur_ready_message")
	assert.Equal(s.T(), response.TargetCluster, "member-123")
}

func (s *TestSignupServiceSuite) TestGetSignupMURGetFails() {
	svc, fakeClient, fakeMURClient := newSignupServiceWithFakeClient()
	returnedErr := errors.New("an error occurred")
	fakeMURClient.MockGet = func(string) (*v1alpha1.MasterUserRecord, error) {
		return nil, returnedErr
	}

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:          "ted",
			CompliantUsername: "ted",
		},
		Status: v1alpha1.UserSignupStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:   v1alpha1.UserSignupComplete,
					Status: apiv1.ConditionTrue,
				},
			},
		},
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), err)

	_, err = svc.GetSignup(userID.String())
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", userID.String()))
}

func newSignupServiceWithFakeClient() (signup.Service, *fake.FakeUserSignupClient, *fake.FakeMasterUserRecordClient) {
	fakeClient := fake.NewFakeUserSignupClient(TestNamespace)
	fakeMURClient := fake.NewFakeMasterUserRecordClient(TestNamespace)
	return &signup.ServiceImpl{
		Namespace:         TestNamespace,
		UserSignups:       fakeClient,
		MasterUserRecords: fakeMURClient,
	}, fakeClient, fakeMURClient
}
