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
	require.False(s.T(), val.Spec.Approved)
}

func (s *TestSignupServiceSuite) TestFailsIfUserSignupNameAlreadyExists() {
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
			Username: "john@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	_, err = svc.CreateUserSignup("john@gmail.com", userID.String())
	require.EqualError(s.T(), err, fmt.Sprintf("usersignups.toolchain.dev.openshift.com \"%s\" already exists", userID.String()))
}

func (s *TestSignupServiceSuite) TestUserSignupGetFails() {
	svc, fakeClient, _ := newSignupServiceWithFakeClient()
	expectedErr := errors.New("an error occurred")

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	fakeClient.MockGet = func(name string) (*v1alpha1.UserSignup, error) {
		if name == userID.String() {
			return nil, expectedErr
		}
		return &v1alpha1.UserSignup{}, nil
	}

	_, err = svc.GetSignup(userID.String())
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
			Username: "bill",
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
	require.Equal(s.T(), "", response.ConsoleURL)
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
			Username: "bill",
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
	require.Equal(s.T(), "", response.ConsoleURL)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusOK() {
	svc, fakeClient, fakeMURClient := newSignupServiceWithFakeClient()

	us := s.newUserSignupComplete()
	err := fakeClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	err = fakeMURClient.Tracker.Add(&v1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ted",
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.MasterUserRecordSpec{
			UserAccounts: []v1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
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
			UserAccounts: []v1alpha1.UserAccountStatusEmbedded{{Cluster: v1alpha1.Cluster{
				Name:       "member-123",
				ConsoleURL: "https://console.member-123.com",
			}}},
		},
	})
	require.NoError(s.T(), err)

	response, err := svc.GetSignup(us.Name)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), "ted", response.Username)
	assert.True(s.T(), response.Status.Ready)
	assert.Equal(s.T(), response.Status.Reason, "mur_ready_reason")
	assert.Equal(s.T(), response.Status.Message, "mur_ready_message")
	assert.Equal(s.T(), response.ConsoleURL, "https://console.member-123.com")
}

func (s *TestSignupServiceSuite) TestGetSignupMURGetFails() {
	svc, fakeClient, fakeMURClient := newSignupServiceWithFakeClient()

	us := s.newUserSignupComplete()
	err := fakeClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	returnedErr := errors.New("an error occurred")
	fakeMURClient.MockGet = func(name string) (*v1alpha1.MasterUserRecord, error) {
		if name == us.Status.CompliantUsername {
			return nil, returnedErr
		}
		return &v1alpha1.MasterUserRecord{}, nil
	}

	_, err = svc.GetSignup(us.Name)
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", us.Name))
}

func (s *TestSignupServiceSuite) TestGetSignupUnknownStatus() {
	svc, fakeClient, fakeMURClient := newSignupServiceWithFakeClient()

	us := s.newUserSignupComplete()
	err := fakeClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	err = fakeMURClient.Tracker.Add(&v1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ted",
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.MasterUserRecordSpec{
			UserAccounts: []v1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
		},
		Status: v1alpha1.MasterUserRecordStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:   v1alpha1.MasterUserRecordReady,
					Status: "blah-blah-blah",
				},
			},
		},
	})
	require.NoError(s.T(), err)

	_, err = svc.GetSignup(us.Name)
	require.EqualError(s.T(), err, "unable to parse readiness status as bool: blah-blah-blah: strconv.ParseBool: parsing \"blah-blah-blah\": invalid syntax")
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

func (s *TestSignupServiceSuite) newUserSignupComplete() *v1alpha1.UserSignup {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	return &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			Username: "ted",
		},
		Status: v1alpha1.UserSignupStatus{
			Conditions: []v1alpha1.Condition{
				{
					Type:   v1alpha1.UserSignupComplete,
					Status: apiv1.ConditionTrue,
				},
			},
			CompliantUsername: "ted",
		},
	}
}
