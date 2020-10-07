package service_test

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	errors2 "k8s.io/apimachinery/pkg/api/errors"

	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
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
	svc, fakeClient, _, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	ctx.Set(context.GivenNameKey, "jane")
	ctx.Set(context.FamilyNameKey, "doe")
	ctx.Set(context.CompanyKey, "red hat")
	userSignup, err := svc.CreateUserSignup(ctx)
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
	require.Equal(s.T(), "jane", val.Spec.GivenName)
	require.Equal(s.T(), "doe", val.Spec.FamilyName)
	require.Equal(s.T(), "red hat", val.Spec.Company)
	require.False(s.T(), val.Spec.Approved)
	require.True(s.T(), val.Spec.VerificationRequired)
	require.Equal(s.T(), "jsmith@gmail.com", val.Annotations[v1alpha1.UserSignupUserEmailAnnotationKey])
	require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[v1alpha1.UserSignupUserEmailHashLabelKey])
	require.Equal(s.T(), "false", val.Labels[v1alpha1.UserSignupApprovedLabelKey])
}

func (s *TestSignupServiceSuite) TestUserWithExcludedDomainEmailSignsUp() {
	svc, fakeClient, _, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@redhat.com")
	ctx.Set(context.GivenNameKey, "jane")
	ctx.Set(context.FamilyNameKey, "smith")
	ctx.Set(context.CompanyKey, "red hat")

	userSignup, err := svc.CreateUserSignup(ctx)
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
	require.False(s.T(), val.Spec.VerificationRequired)
}

func (s *TestSignupServiceSuite) TestFailsIfUserSignupNameAlreadyExists() {
	svc, fakeClient, _, _ := newSignupServiceWithFakeClient()
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "john@gmail.com",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username: "john@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	_, err = svc.CreateUserSignup(ctx)

	require.EqualError(s.T(), err, fmt.Sprintf("usersignups.toolchain.dev.openshift.com \"%s\" already exists", userID.String()))
}

func (s *TestSignupServiceSuite) TestFailsIfUserBanned() {
	svc, _, _, fakeBannedUserClient := newSignupServiceWithFakeClient()
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeBannedUserClient.Tracker.Add(&v1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: TestNamespace,
			Labels: map[string]string{
				v1alpha1.BannedUserEmailHashLabelKey: "a7b1b413c1cbddbcd19a51222ef8e20a",
			},
		},
		Spec: v1alpha1.BannedUserSpec{
			Email: "jsmith@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	_, err = svc.CreateUserSignup(ctx)

	require.Error(s.T(), err)
	require.IsType(s.T(), &errors2.StatusError{}, err)
	errStatus := err.(*errors2.StatusError).ErrStatus
	require.Equal(s.T(), "Failure", errStatus.Status)
	require.Equal(s.T(), "user has been banned", errStatus.Message)
	require.Equal(s.T(), v1.StatusReasonBadRequest, errStatus.Reason)
}

func (s *TestSignupServiceSuite) TestPhoneNumberAlreadyInUseBannedUser() {
	svc, _, _, fakeBannedUserClient := newSignupServiceWithFakeClient()
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeBannedUserClient.Tracker.Add(&v1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: TestNamespace,
			Labels: map[string]string{
				v1alpha1.BannedUserEmailHashLabelKey:       "a7b1b413c1cbddbcd19a51222ef8e20a",
				v1alpha1.BannedUserPhoneNumberHashLabelKey: "a97abba6f48335650ea0523aedc059c7",
			},
		},
		Spec: v1alpha1.BannedUserSpec{
			Email: "jane.doe@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	err = svc.PhoneNumberAlreadyInUse(bannedUserID.String(), "1", "2268213044")
	require.Error(s.T(), err)
	require.Equal(s.T(), "cannot re-register with phone number:phone number already in use", err.Error())
}

func (s *TestSignupServiceSuite) TestPhoneNumberAlreadyInUseUserSignup() {
	svc, fakeuserSignupClient, _, _ := newSignupServiceWithFakeClient()
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeuserSignupClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
			Labels: map[string]string{
				v1alpha1.UserSignupUserEmailHashLabelKey: "a7b1b413c1cbddbcd19a51222ef8e20a",
				v1alpha1.UserSignupUserPhoneHashLabelKey: "a97abba6f48335650ea0523aedc059c7",
			},
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	newUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	err = svc.PhoneNumberAlreadyInUse(newUserID.String(), "1", "2268213044")
	require.Error(s.T(), err)
	require.Equal(s.T(), "cannot re-register with phone number:phone number already in use", err.Error())
}

func (s *TestSignupServiceSuite) TestOKIfOtherUserBanned() {
	svc, fakeClient, _, fakeBannedUserClient := newSignupServiceWithFakeClient()
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeBannedUserClient.Tracker.Add(&v1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: TestNamespace,
			Labels: map[string]string{
				v1alpha1.BannedUserEmailHashLabelKey: "1df66fbb427ff7e64ac46af29cc74b71",
			},
		},
		Spec: v1alpha1.BannedUserSpec{
			Email: "jane.doe@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	userSignup, err := svc.CreateUserSignup(ctx)
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
	require.Equal(s.T(), "", val.Spec.GivenName)
	require.Equal(s.T(), "", val.Spec.FamilyName)
	require.Equal(s.T(), "", val.Spec.Company)
	require.False(s.T(), val.Spec.Approved)
	require.Equal(s.T(), "jsmith@gmail.com", val.Annotations[v1alpha1.UserSignupUserEmailAnnotationKey])
	require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[v1alpha1.UserSignupUserEmailHashLabelKey])
	require.Equal(s.T(), "false", val.Labels[v1alpha1.UserSignupApprovedLabelKey])
}

func (s *TestSignupServiceSuite) TestGetUserSignupFails() {
	svc, fakeClient, _, _ := newSignupServiceWithFakeClient()
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
	svc, _, _, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	signup, err := svc.GetSignup(userID.String())
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusNotComplete() {
	svc, fakeClient, _, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "bill",
			VerificationRequired: true,
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
	require.Equal(s.T(), "", response.CompliantUsername)
	require.False(s.T(), response.Status.Ready)
	require.Equal(s.T(), response.Status.Reason, "test_reason")
	require.Equal(s.T(), response.Status.Message, "test_message")
	require.True(s.T(), response.Status.VerificationRequired)
	require.Equal(s.T(), "", response.ConsoleURL)
	require.Equal(s.T(), "", response.CheDashboardURL)
}

func (s *TestSignupServiceSuite) TestGetSignupNoStatusNotCompleteCondition() {
	svc, fakeClient, _, _ := newSignupServiceWithFakeClient()

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = fakeClient.Tracker.Add(&v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "bill",
			VerificationRequired: true,
		},
		Status: v1alpha1.UserSignupStatus{},
	})
	require.NoError(s.T(), err)

	response, err := svc.GetSignup(userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), "bill", response.Username)
	require.Equal(s.T(), "", response.CompliantUsername)
	require.False(s.T(), response.Status.Ready)
	require.Equal(s.T(), "PendingApproval", response.Status.Reason)
	require.True(s.T(), response.Status.VerificationRequired)
	require.Equal(s.T(), "", response.Status.Message)
	require.Equal(s.T(), "", response.ConsoleURL)
	require.Equal(s.T(), "", response.CheDashboardURL)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusOK() {
	svc, fakeClient, fakeMURClient, _ := newSignupServiceWithFakeClient()

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
				Name:            "member-123",
				ConsoleURL:      "https://console.member-123.com",
				CheDashboardURL: "http://che-toolchain-che.member-123.com",
			}}},
		},
	})
	require.NoError(s.T(), err)

	response, err := svc.GetSignup(us.Name)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), "ted@domain.com", response.Username)
	require.Equal(s.T(), "ted", response.CompliantUsername)
	assert.True(s.T(), response.Status.Ready)
	assert.Equal(s.T(), response.Status.Reason, "mur_ready_reason")
	assert.Equal(s.T(), response.Status.Message, "mur_ready_message")
	assert.False(s.T(), response.Status.VerificationRequired)
	assert.Equal(s.T(), response.ConsoleURL, "https://console.member-123.com")
	assert.Equal(s.T(), response.CheDashboardURL, "http://che-toolchain-che.member-123.com")
}

func (s *TestSignupServiceSuite) TestGetSignupMURGetFails() {
	svc, fakeClient, fakeMURClient, _ := newSignupServiceWithFakeClient()

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
	svc, fakeClient, fakeMURClient, _ := newSignupServiceWithFakeClient()

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

func (s *TestSignupServiceSuite) TestGetUserSignup() {
	svc, fakeClient, _, _ := newSignupServiceWithFakeClient()

	s.Run("getusersignup ok", func() {
		us := s.newUserSignupComplete()
		err := fakeClient.Tracker.Add(us)
		require.NoError(s.T(), err)

		val, err := svc.GetUserSignup(us.Name)
		require.NoError(s.T(), err)
		require.Equal(s.T(), us.Name, val.Name)
	})

	s.Run("getusersignup returns error", func() {
		fakeClient.MockGet = func(s string) (userSignup *v1alpha1.UserSignup, e error) {
			return nil, errors.New("get failed")
		}

		val, err := svc.GetUserSignup("foo")
		require.Error(s.T(), err)
		require.Equal(s.T(), "get failed", err.Error())
		require.Nil(s.T(), val)
	})

	s.Run("getusersignup with unknown user", func() {
		fakeClient.MockGet = nil

		val, err := svc.GetUserSignup("unknown")
		require.True(s.T(), errors2.IsNotFound(err))
		require.Nil(s.T(), val)
	})
}

func (s *TestSignupServiceSuite) TestUpdateUserSignup() {
	svc, fakeClient, _, _ := newSignupServiceWithFakeClient()

	us := s.newUserSignupComplete()
	err := fakeClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	s.Run("updateusersignup ok", func() {
		val, err := svc.GetUserSignup(us.Name)
		require.NoError(s.T(), err)

		val.Spec.FamilyName = "Johnson"

		updated, err := svc.UpdateUserSignup(val)
		require.NoError(s.T(), err)

		require.Equal(s.T(), val.Spec.FamilyName, updated.Spec.FamilyName)
	})

	s.Run("updateusersignup returns error", func() {
		fakeClient.MockUpdate = func(userSignup2 *v1alpha1.UserSignup) (userSignup *v1alpha1.UserSignup, e error) {
			return nil, errors.New("update failed")
		}

		val, err := svc.GetUserSignup(us.Name)
		require.NoError(s.T(), err)

		updated, err := svc.UpdateUserSignup(val)
		require.Error(s.T(), err)
		require.Equal(s.T(), "update failed", err.Error())
		require.Nil(s.T(), updated)
	})
}

type FakeSignupServiceConfiguration struct {
	namespace                    string
	verificationEnabled          bool
	excludedDomains              []string
	verificationCodeExpiresInMin int
}

func (c *FakeSignupServiceConfiguration) GetNamespace() string {
	return c.namespace
}

func (c *FakeSignupServiceConfiguration) GetVerificationEnabled() bool {
	return c.verificationEnabled
}

func (c *FakeSignupServiceConfiguration) GetVerificationExcludedEmailDomains() []string {
	return c.excludedDomains
}

func (c *FakeSignupServiceConfiguration) GetVerificationCodeExpiresInMin() int {
	return c.verificationCodeExpiresInMin
}

func newSignupServiceWithFakeClient() (signup.Service, *fake.FakeUserSignupClient, *fake.FakeMasterUserRecordClient, *fake.FakeBannedUserClient) {
	fakeClient := fake.NewFakeUserSignupClient(TestNamespace)
	fakeMURClient := fake.NewFakeMasterUserRecordClient(TestNamespace)
	fakeBannedUserClient := fake.NewFakeBannedUserClient(TestNamespace)
	return &ServiceImpl{
		Namespace:         TestNamespace,
		UserSignups:       fakeClient,
		MasterUserRecords: fakeMURClient,
		BannedUsers:       fakeBannedUserClient,
		Config: &FakeSignupServiceConfiguration{
			namespace:           "test",
			verificationEnabled: true,
			excludedDomains:     []string{"redhat.com"},
		},
	}, fakeClient, fakeMURClient, fakeBannedUserClient
}

func (s *TestSignupServiceSuite) newUserSignupComplete() *v1alpha1.UserSignup {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	return &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "ted@domain.com",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username: "ted@domain.com",
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
