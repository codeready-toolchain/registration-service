package service_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash/crc32"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/codeready-toolchain/toolchain-common/pkg/condition"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/signup/service"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"

	"k8s.io/apimachinery/pkg/runtime/schema"

	test2 "github.com/codeready-toolchain/toolchain-common/pkg/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/gin-gonic/gin"

	errors2 "k8s.io/apimachinery/pkg/api/errors"

	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/test"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	TestNamespace = "test-namespace-123"

	activationCodeCharset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

type TestSignupServiceSuite struct {
	test.UnitTestSuite
}

func TestRunSignupServiceSuite(t *testing.T) {
	suite.Run(t, &TestSignupServiceSuite{test.UnitTestSuite{}})
}

func (s *TestSignupServiceSuite) ServiceConfiguration(namespace string, verificationEnabled bool,
	excludedDomains string, verificationCodeExpiresInMin int) {

	test2.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, namespace)

	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().Enabled(verificationEnabled).
			Verification().CodeExpiresInMin(verificationCodeExpiresInMin).
			Verification().ExcludedEmailDomains(excludedDomains))
}

func (s *TestSignupServiceSuite) TestSignup() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	assertUserSignupExists := func(userSignup *toolchainv1alpha1.UserSignup, userID string) (schema.GroupVersionResource, toolchainv1alpha1.UserSignup) {
		require.NotNil(s.T(), userSignup)

		gvk, err := apiutil.GVKForObject(userSignup, s.FakeUserSignupClient.Scheme)
		require.NoError(s.T(), err)
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)

		values, err := s.FakeUserSignupClient.Tracker.List(gvr, gvk, configuration.Namespace())
		require.NoError(s.T(), err)

		userSignups := values.(*toolchainv1alpha1.UserSignupList)
		require.NotEmpty(s.T(), userSignups.Items)
		require.Len(s.T(), userSignups.Items, 1)

		val := userSignups.Items[0]
		require.Equal(s.T(), configuration.Namespace(), val.Namespace)
		require.Equal(s.T(), userID, val.Name)
		require.Equal(s.T(), userID, val.Spec.Userid)
		require.Equal(s.T(), "jsmith", val.Spec.Username)
		require.Equal(s.T(), "jane", val.Spec.GivenName)
		require.Equal(s.T(), "doe", val.Spec.FamilyName)
		require.Equal(s.T(), "red hat", val.Spec.Company)
		require.False(s.T(), states.Approved(&val))
		require.True(s.T(), states.VerificationRequired(&val))
		require.Equal(s.T(), "jsmith@gmail.com", val.Annotations[toolchainv1alpha1.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])

		return gvr, val
	}

	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.OriginalSubKey, "original-sub-value")
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	ctx.Set(context.GivenNameKey, "jane")
	ctx.Set(context.FamilyNameKey, "doe")
	ctx.Set(context.CompanyKey, "red hat")

	// when
	userSignup, err := s.Application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // at this point, the annotation is not set
	require.Equal(s.T(), "original-sub-value", userSignup.Spec.OriginalSub)

	gvr, existing := assertUserSignupExists(userSignup, userID.String())

	s.Run("deactivate and reactivate again", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		deactivatedUS.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey] = "2" // assume the user was activated 2 times already
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = deactivated()
		err := s.FakeUserSignupClient.Tracker.Update(gvr, deactivatedUS, configuration.Namespace())
		require.NoError(s.T(), err)

		// when
		deactivatedUS, err = s.Application.SignupService().Signup(ctx)

		// then
		require.NoError(s.T(), err)
		assertUserSignupExists(deactivatedUS, userID.String())
		assert.NotEmpty(s.T(), deactivatedUS.ResourceVersion)
		assert.Equal(s.T(), "2", deactivatedUS.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // value was preserved
	})

	s.Run("deactivate and reactivate with missing annotation", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = deactivated()
		// also, alter the activation counter annotation
		delete(deactivatedUS.Annotations, toolchainv1alpha1.UserSignupActivationCounterAnnotationKey)
		err := s.FakeUserSignupClient.Tracker.Update(gvr, deactivatedUS, configuration.Namespace())
		require.NoError(s.T(), err)

		// when
		userSignup, err := s.Application.SignupService().Signup(ctx)

		// then
		require.NoError(s.T(), err)
		assertUserSignupExists(userSignup, userID.String())
		assert.NotEmpty(s.T(), userSignup.ResourceVersion)
		assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // was initially missing, and was not set
	})

	s.Run("deactivate and try to reactivate but reactivation fails", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = deactivated()
		err := s.FakeUserSignupClient.Tracker.Update(gvr, deactivatedUS, configuration.Namespace())
		require.NoError(s.T(), err)
		s.FakeUserSignupClient.MockUpdate = func(signup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
			if signup.Name == userID.String() {
				return nil, errors.New("an error occurred")
			}
			return &toolchainv1alpha1.UserSignup{}, nil
		}

		// when
		_, err = s.Application.SignupService().Signup(ctx)

		// then
		require.EqualError(s.T(), err, "an error occurred")
	})
}

func (s *TestSignupServiceSuite) TestUserSignupWithInvalidSubjectPrefix() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	subject := fmt.Sprintf("-%s", userID.String())

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "sjones")
	ctx.Set(context.SubKey, subject)
	ctx.Set(context.EmailKey, "sjones@gmail.com")
	ctx.Set(context.GivenNameKey, "sam")
	ctx.Set(context.FamilyNameKey, "jones")
	ctx.Set(context.CompanyKey, "red hat")

	// when
	userSignup, err := s.Application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)

	gvk, err := apiutil.GVKForObject(userSignup, s.FakeUserSignupClient.Scheme)
	require.NoError(s.T(), err)
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	values, err := s.FakeUserSignupClient.Tracker.List(gvr, gvk, configuration.Namespace())
	require.NoError(s.T(), err)

	userSignups := values.(*toolchainv1alpha1.UserSignupList)
	require.NotEmpty(s.T(), userSignups.Items)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]

	// Confirm that the UserSignup.Name value has been prefixed correctly
	expected := fmt.Sprintf("%x%s", crc32.Checksum([]byte(subject), crc32.IEEETable), subject)
	require.Equal(s.T(), expected, val.Name)
	require.False(s.T(), strings.HasPrefix(val.Name, "-"))
}

func (s *TestSignupServiceSuite) TestEncodeUserID() {
	s.Run("test valid user ID unchanged", func() {
		userID := "abcde-12345"
		encoded := service.EncodeUserID(userID)
		require.Equal(s.T(), userID, encoded)
	})
	s.Run("test user ID with invalid characters", func() {
		userID := "abcde\\*-12345"
		encoded := service.EncodeUserID(userID)
		require.Equal(s.T(), "c0177ca4-abcde-12345", encoded)
	})
	s.Run("test user ID with invalid prefix", func() {
		userID := "-1234567"
		encoded := service.EncodeUserID(userID)
		require.Equal(s.T(), "ca3e1e0f-1234567", encoded)
	})
	s.Run("test user ID that exceeds max length", func() {
		userID := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-01234567890123456789"
		encoded := service.EncodeUserID(userID)
		require.Equal(s.T(), "e3632025-0123456789abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr", encoded)
	})
	s.Run("test user ID with colon separator", func() {
		userID := "abc:xyz"
		encoded := service.EncodeUserID(userID)
		require.Equal(s.T(), "a05a4053-abcxyz", encoded)
	})
}

func (s *TestSignupServiceSuite) TestUserWithExcludedDomainEmailSignsUp() {
	s.ServiceConfiguration(TestNamespace, true, "redhat.com", 5)

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

	userSignup, err := s.Application.SignupService().Signup(ctx)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	gvk, err := apiutil.GVKForObject(userSignup, s.FakeUserSignupClient.Scheme)
	require.NoError(s.T(), err)
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	values, err := s.FakeUserSignupClient.Tracker.List(gvr, gvk, TestNamespace)
	require.NoError(s.T(), err)

	userSignups := values.(*toolchainv1alpha1.UserSignupList)
	require.NotEmpty(s.T(), userSignups.Items)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.False(s.T(), states.VerificationRequired(&val))
}

func (s *TestSignupServiceSuite) TestCRTAdminUserSignup() {
	s.ServiceConfiguration(TestNamespace, true, "redhat.com", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith-crtadmin")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@redhat.com")
	ctx.Set(context.GivenNameKey, "jane")
	ctx.Set(context.FamilyNameKey, "smith")
	ctx.Set(context.CompanyKey, "red hat")

	userSignup, err := s.Application.SignupService().Signup(ctx)
	require.Error(s.T(), err)
	require.Equal(s.T(), "forbidden: failed to create usersignup for jsmith-crtadmin", err.Error())
	require.Nil(s.T(), userSignup)
}

func (s *TestSignupServiceSuite) TestFailsIfUserSignupNameAlreadyExists() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	err = s.FakeUserSignupClient.Tracker.Add(&toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "john@gmail.com",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "john@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	_, err = s.Application.SignupService().Signup(ctx)

	require.EqualError(s.T(), err, fmt.Sprintf("Operation cannot be fulfilled on  \"\": UserSignup [id: %s; username: jsmith]. Unable to create UserSignup because there is already an active UserSignup with such ID", userID.String()))
}

func (s *TestSignupServiceSuite) TestFailsIfUserBanned() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = s.FakeBannedUserClient.Tracker.Add(&toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: TestNamespace,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: "a7b1b413c1cbddbcd19a51222ef8e20a",
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "jsmith@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	// when
	_, err = s.Application.SignupService().Signup(ctx)

	// then
	require.Error(s.T(), err)
	require.IsType(s.T(), &errors2.StatusError{}, err)
	errStatus := err.(*errors2.StatusError).ErrStatus
	require.Equal(s.T(), "Failure", errStatus.Status)
	require.Equal(s.T(), "forbidden: user has been banned", errStatus.Message)
	require.Equal(s.T(), v1.StatusReasonForbidden, errStatus.Reason)
}

func (s *TestSignupServiceSuite) TestPhoneNumberAlreadyInUseBannedUser() {
	s.ServiceConfiguration(TestNamespace, true, "redhat.com", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = s.FakeBannedUserClient.Tracker.Add(&toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: TestNamespace,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey:       "a7b1b413c1cbddbcd19a51222ef8e20a",
				toolchainv1alpha1.BannedUserPhoneNumberHashLabelKey: "fd276563a8232d16620da8ec85d0575f",
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "jane.doe@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	err = s.Application.SignupService().PhoneNumberAlreadyInUse(bannedUserID.String(), "+12268213044")
	require.Error(s.T(), err)
	require.Equal(s.T(), "cannot re-register with phone number:phone number already in use", err.Error())
}

func (s *TestSignupServiceSuite) TestPhoneNumberAlreadyInUseUserSignup() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = s.FakeUserSignupClient.Tracker.Add(&toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailHashLabelKey: "a7b1b413c1cbddbcd19a51222ef8e20a",
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "fd276563a8232d16620da8ec85d0575f",
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
	err = s.Application.SignupService().PhoneNumberAlreadyInUse(newUserID.String(), "+12268213044")
	require.Error(s.T(), err)
	require.Equal(s.T(), "cannot re-register with phone number:phone number already in use", err.Error())
}

func (s *TestSignupServiceSuite) TestOKIfOtherUserBanned() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = s.FakeBannedUserClient.Tracker.Add(&toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: TestNamespace,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: "1df66fbb427ff7e64ac46af29cc74b71",
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "jane.doe@gmail.com",
		},
	})
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	userSignup, err := s.Application.SignupService().Signup(ctx)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	gvk, err := apiutil.GVKForObject(userSignup, s.FakeUserSignupClient.Scheme)
	require.NoError(s.T(), err)
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	values, err := s.FakeUserSignupClient.Tracker.List(gvr, gvk, TestNamespace)
	require.NoError(s.T(), err)

	userSignups := values.(*toolchainv1alpha1.UserSignupList)
	require.NotEmpty(s.T(), userSignups.Items)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.Equal(s.T(), TestNamespace, val.Namespace)
	require.Equal(s.T(), userID.String(), val.Name)
	require.Equal(s.T(), "jsmith", val.Spec.Username)
	require.Equal(s.T(), "", val.Spec.GivenName)
	require.Equal(s.T(), "", val.Spec.FamilyName)
	require.Equal(s.T(), "", val.Spec.Company)
	require.False(s.T(), states.Approved(&val))
	require.Equal(s.T(), "jsmith@gmail.com", val.Annotations[toolchainv1alpha1.UserSignupUserEmailAnnotationKey])
	require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])
}

func (s *TestSignupServiceSuite) TestGetUserSignupFails() {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	s.FakeUserSignupClient.MockGet = func(name string) (*toolchainv1alpha1.UserSignup, error) {
		if name == userID.String() {
			return nil, errors.New("an error occurred")
		}
		return &toolchainv1alpha1.UserSignup{}, nil
	}

	_, err = s.Application.SignupService().GetSignup(userID.String())
	require.EqualError(s.T(), err, "an error occurred")
}

func (s *TestSignupServiceSuite) TestGetSignupNotFound() {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	signup, err := s.Application.SignupService().GetSignup(userID.String())
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusNotComplete() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	userSignup := toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "bill",
		},
		Status: toolchainv1alpha1.UserSignupStatus{
			Conditions: []toolchainv1alpha1.Condition{
				{
					Type:    toolchainv1alpha1.UserSignupComplete,
					Status:  apiv1.ConditionFalse,
					Reason:  "test_reason",
					Message: "test_message",
				},
				{
					Type:   toolchainv1alpha1.UserSignupApproved,
					Status: apiv1.ConditionTrue,
					Reason: toolchainv1alpha1.UserSignupApprovedAutomaticallyReason,
				},
			},
		},
	}
	states.SetVerificationRequired(&userSignup, true)

	err = s.FakeUserSignupClient.Tracker.Add(&userSignup)
	require.NoError(s.T(), err)

	response, err := s.Application.SignupService().GetSignup(userID.String())
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
	require.Equal(s.T(), "", response.APIEndpoint)
	require.Equal(s.T(), "", response.ClusterName)
}

func (s *TestSignupServiceSuite) TestGetSignupNoStatusNotCompleteCondition() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	noCondition := toolchainv1alpha1.UserSignupStatus{}
	pendingApproval := toolchainv1alpha1.UserSignupStatus{
		Conditions: []toolchainv1alpha1.Condition{
			{
				Type:   toolchainv1alpha1.UserSignupApproved,
				Status: apiv1.ConditionFalse,
				Reason: toolchainv1alpha1.UserSignupPendingApprovalReason,
			},
		},
	}
	noClusterApproval := toolchainv1alpha1.UserSignupStatus{
		Conditions: []toolchainv1alpha1.Condition{
			{
				Type:   toolchainv1alpha1.UserSignupApproved,
				Status: apiv1.ConditionFalse,
				Reason: toolchainv1alpha1.UserSignupPendingApprovalReason,
			},
			{
				Type:   toolchainv1alpha1.UserSignupComplete,
				Status: apiv1.ConditionFalse,
				Reason: toolchainv1alpha1.UserSignupNoClusterAvailableReason,
			},
		},
	}

	for _, status := range []toolchainv1alpha1.UserSignupStatus{noCondition, pendingApproval, noClusterApproval} {
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)

		userSignup := toolchainv1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      userID.String(),
				Namespace: TestNamespace,
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				Username: "bill",
			},
			Status: status,
		}

		states.SetVerificationRequired(&userSignup, true)

		err = s.FakeUserSignupClient.Tracker.Add(&userSignup)
		require.NoError(s.T(), err)

		response, err := s.Application.SignupService().GetSignup(userID.String())
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
		require.Equal(s.T(), "", response.APIEndpoint)
		require.Equal(s.T(), "", response.ClusterName)
	}
}

func (s *TestSignupServiceSuite) TestGetSignupDeactivated() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupCompleteWithReason("Deactivated")
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	signup, err := s.Application.SignupService().GetSignup(us.Name)
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusOK() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	err = s.FakeMasterUserRecordClient.Tracker.Add(s.newProvisionedMUR())
	require.NoError(s.T(), err)

	err = s.FakeToolchainStatusClient.Tracker.Add(&toolchainv1alpha1.ToolchainStatus{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "toolchain-status",
			Namespace: TestNamespace,
		},
		Status: toolchainv1alpha1.ToolchainStatusStatus{
			Members: []toolchainv1alpha1.Member{
				{
					ClusterName: "member-1",
					ApiEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL:      "https://console.member-1.com",
							CheDashboardURL: "http://che-toolchain-che.member-1.com",
						},
					},
				},
				{
					ClusterName: "member-123",
					ApiEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL:      "https://console.member-123.com",
							CheDashboardURL: "http://che-toolchain-che.member-123.com",
						},
					},
				},
			},
		},
	})
	require.NoError(s.T(), err)

	response, err := s.Application.SignupService().GetSignup(us.Name)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), "ted@domain.com", response.Username)
	require.Equal(s.T(), "ted", response.CompliantUsername)
	assert.True(s.T(), response.Status.Ready)
	assert.Equal(s.T(), "mur_ready_reason", response.Status.Reason)
	assert.Equal(s.T(), "mur_ready_message", response.Status.Message)
	assert.False(s.T(), response.Status.VerificationRequired)
	assert.Equal(s.T(), "https://console.member-123.com", response.ConsoleURL)
	assert.Equal(s.T(), "http://che-toolchain-che.member-123.com", response.CheDashboardURL)
	assert.Equal(s.T(), "http://api.devcluster.openshift.com", response.APIEndpoint)
	assert.Equal(s.T(), "member-123", response.ClusterName)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusFailGetToolchainStatus() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	err = s.FakeMasterUserRecordClient.Tracker.Add(s.newProvisionedMUR())
	require.NoError(s.T(), err)

	_, err = s.Application.SignupService().GetSignup(us.Name)
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s: toolchainstatuses.toolchain.dev.openshift.com \"toolchain-status\" not found", us.Name))
}

func (s *TestSignupServiceSuite) TestGetSignupMURGetFails() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	returnedErr := errors.New("an error occurred")
	s.FakeMasterUserRecordClient.MockGet = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
		if name == us.Status.CompliantUsername {
			return nil, returnedErr
		}
		return &toolchainv1alpha1.MasterUserRecord{}, nil
	}

	_, err = s.Application.SignupService().GetSignup(us.Name)
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", us.Name))
}

func (s *TestSignupServiceSuite) TestGetSignupUnknownStatus() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	err = s.FakeMasterUserRecordClient.Tracker.Add(&toolchainv1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ted",
			Namespace: TestNamespace,
		},
		Spec: toolchainv1alpha1.MasterUserRecordSpec{
			UserAccounts: []toolchainv1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
		},
		Status: toolchainv1alpha1.MasterUserRecordStatus{
			Conditions: []toolchainv1alpha1.Condition{
				{
					Type:   toolchainv1alpha1.MasterUserRecordReady,
					Status: "blah-blah-blah",
				},
			},
		},
	})
	require.NoError(s.T(), err)

	_, err = s.Application.SignupService().GetSignup(us.Name)
	require.EqualError(s.T(), err, "unable to parse readiness status as bool: blah-blah-blah: strconv.ParseBool: parsing \"blah-blah-blah\": invalid syntax")
}

func (s *TestSignupServiceSuite) TestGetUserSignup() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	s.Run("getusersignup ok", func() {
		us := s.newUserSignupComplete()
		err := s.FakeUserSignupClient.Tracker.Add(us)
		require.NoError(s.T(), err)

		val, err := s.Application.SignupService().GetUserSignup(us.Name)
		require.NoError(s.T(), err)
		require.Equal(s.T(), us.Name, val.Name)
	})

	s.Run("getusersignup returns error", func() {
		s.FakeUserSignupClient.MockGet = func(s string) (userSignup *toolchainv1alpha1.UserSignup, e error) {
			return nil, errors.New("get failed")
		}

		val, err := s.Application.SignupService().GetUserSignup("foo")
		require.Error(s.T(), err)
		require.Equal(s.T(), "get failed", err.Error())
		require.Nil(s.T(), val)
	})

	s.Run("getusersignup with unknown user", func() {
		s.FakeUserSignupClient.MockGet = nil

		val, err := s.Application.SignupService().GetUserSignup("unknown")
		require.True(s.T(), errors2.IsNotFound(err))
		require.Nil(s.T(), val)
	})
}

func (s *TestSignupServiceSuite) TestUpdateUserSignup() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	s.Run("updateusersignup ok", func() {
		val, err := s.Application.SignupService().GetUserSignup(us.Name)
		require.NoError(s.T(), err)

		val.Spec.FamilyName = "Johnson"

		updated, err := s.Application.SignupService().UpdateUserSignup(val)
		require.NoError(s.T(), err)

		require.Equal(s.T(), val.Spec.FamilyName, updated.Spec.FamilyName)
	})

	s.Run("updateusersignup returns error", func() {
		s.FakeUserSignupClient.MockUpdate = func(userSignup2 *toolchainv1alpha1.UserSignup) (userSignup *toolchainv1alpha1.UserSignup, e error) {
			return nil, errors.New("update failed")
		}

		val, err := s.Application.SignupService().GetUserSignup(us.Name)
		require.NoError(s.T(), err)

		updated, err := s.Application.SignupService().UpdateUserSignup(val)
		require.Error(s.T(), err)
		require.Equal(s.T(), "update failed", err.Error())
		require.Nil(s.T(), updated)
	})
}

func (s *TestSignupServiceSuite) TestRegisterByActivationCode() {

	s.Run("Test register user by active activation code ok", func() {
		event, err := s.newToolchainEvent(10, false, ToolchainEventOptionActive)
		require.NoError(s.T(), err)

		err = s.FakeToolchainEventClient.Tracker.Add(event)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// given
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.UsernameKey, "jackjones")
		ctx.Set(context.SubKey, userID.String())
		ctx.Set(context.EmailKey, "jjones1999@gmail.com")
		ctx.Set(context.GivenNameKey, "jack")
		ctx.Set(context.FamilyNameKey, "jones")
		ctx.Set(context.CompanyKey, "red hat")

		// when
		userSignup, err := s.Application.SignupService().Activate(ctx, event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// then
		require.NoError(s.T(), err)
		require.False(s.T(), states.VerificationRequired(userSignup))
		require.Equal(s.T(), event.Name, userSignup.Labels[toolchainv1alpha1.UserSignupToolchainEventLabelKey])
	})

	s.Run("Test register user by active activation code but verification still required ok", func() {
		event, err := s.newToolchainEvent(10, true, ToolchainEventOptionActive)
		require.NoError(s.T(), err)

		err = s.FakeToolchainEventClient.Tracker.Add(event)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// given
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.UsernameKey, "bobjones")
		ctx.Set(context.SubKey, userID.String())
		ctx.Set(context.EmailKey, "bobjones1980@gmail.com")
		ctx.Set(context.GivenNameKey, "bob")
		ctx.Set(context.FamilyNameKey, "jones")
		ctx.Set(context.CompanyKey, "red hat")

		// when
		userSignup, err := s.Application.SignupService().Activate(ctx, event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// then
		require.NoError(s.T(), err)
		require.True(s.T(), states.VerificationRequired(userSignup))
		require.Equal(s.T(), event.Name, userSignup.Labels[toolchainv1alpha1.UserSignupToolchainEventLabelKey])
	})

	s.Run("Test register user by inactive activation code fails", func() {
		event, err := s.newToolchainEvent(10, false, ToolchainEventOptionNotActive)
		require.NoError(s.T(), err)

		err = s.FakeToolchainEventClient.Tracker.Add(event)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// given
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.UsernameKey, "jilljones")
		ctx.Set(context.SubKey, userID.String())
		ctx.Set(context.EmailKey, "jilljones1992@gmail.com")
		ctx.Set(context.GivenNameKey, "jill")
		ctx.Set(context.FamilyNameKey, "jones")
		ctx.Set(context.CompanyKey, "red hat")

		// when
		userSignup, err := s.Application.SignupService().Activate(ctx, event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// then
		require.Error(s.T(), err)
		require.Equal(s.T(), "invalid activation code", err.Error())
		require.Nil(s.T(), userSignup)
	})

	s.Run("Test register user with expired activation code fails", func() {
		event, err := s.newToolchainEvent(10, false, ToolchainEventOptionExpired)
		require.NoError(s.T(), err)

		err = s.FakeToolchainEventClient.Tracker.Add(event)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// given
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.UsernameKey, "hankjohnson")
		ctx.Set(context.SubKey, userID.String())
		ctx.Set(context.EmailKey, "hankjohnson@gmail.com")
		ctx.Set(context.GivenNameKey, "hank")
		ctx.Set(context.FamilyNameKey, "johnson")
		ctx.Set(context.CompanyKey, "red hat")

		// when
		userSignup, err := s.Application.SignupService().Activate(ctx, event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// then
		require.Error(s.T(), err)
		require.Equal(s.T(), "invalid activation code", err.Error())
		require.Nil(s.T(), userSignup)
	})

	s.Run("Test register user with multiple events with same code fails", func() {
		event, err := s.newToolchainEvent(10, false, ToolchainEventOptionActive)
		require.NoError(s.T(), err)
		event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey] = "AAA-BBB-CCC"

		event2, err := s.newToolchainEvent(10, false, ToolchainEventOptionActive)
		require.NoError(s.T(), err)
		event2.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey] = "AAA-BBB-CCC"

		err = s.FakeToolchainEventClient.Tracker.Add(event)
		require.NoError(s.T(), err)

		err = s.FakeToolchainEventClient.Tracker.Add(event2)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])

		// given
		userID, err := uuid.NewV4()
		require.NoError(s.T(), err)

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.UsernameKey, "hankjohnson")
		ctx.Set(context.SubKey, userID.String())
		ctx.Set(context.EmailKey, "hankjohnson@gmail.com")
		ctx.Set(context.GivenNameKey, "hank")
		ctx.Set(context.FamilyNameKey, "johnson")
		ctx.Set(context.CompanyKey, "red hat")

		// when
		userSignup, err := s.Application.SignupService().Activate(ctx, "AAA-BBB-CCC")

		// then
		require.Error(s.T(), err)
		require.Equal(s.T(), "Internal error occurred: multiple matching ToolchainEvent resources found with duplicate activation code", err.Error())
		require.Nil(s.T(), userSignup)
	})

	s.Run("Test user cannot register after code exhausted", func() {
		event, err := s.newToolchainEvent(2, false, ToolchainEventOptionActive)
		require.NoError(s.T(), err)

		err = s.FakeToolchainEventClient.Tracker.Add(event)
		require.NoError(s.T(), err)

		// given
		userID1, err := uuid.NewV4()
		require.NoError(s.T(), err)

		rr := httptest.NewRecorder()
		ctx1, _ := gin.CreateTestContext(rr)
		ctx1.Set(context.UsernameKey, "user1")
		ctx1.Set(context.SubKey, userID1.String())
		ctx1.Set(context.EmailKey, "user1@gmail.com")
		ctx1.Set(context.GivenNameKey, "user")
		ctx1.Set(context.FamilyNameKey, "one")
		ctx1.Set(context.CompanyKey, "red hat")

		userID2, err := uuid.NewV4()
		require.NoError(s.T(), err)

		ctx2, _ := gin.CreateTestContext(rr)
		ctx2.Set(context.UsernameKey, "user2")
		ctx2.Set(context.SubKey, userID2.String())
		ctx2.Set(context.EmailKey, "user2@gmail.com")
		ctx2.Set(context.GivenNameKey, "user")
		ctx2.Set(context.FamilyNameKey, "two")
		ctx2.Set(context.CompanyKey, "red hat")

		userID3, err := uuid.NewV4()
		require.NoError(s.T(), err)

		ctx3, _ := gin.CreateTestContext(rr)
		ctx3.Set(context.UsernameKey, "user3")
		ctx3.Set(context.SubKey, userID3.String())
		ctx3.Set(context.EmailKey, "user3@gmail.com")
		ctx3.Set(context.GivenNameKey, "user")
		ctx3.Set(context.FamilyNameKey, "three")
		ctx3.Set(context.CompanyKey, "red hat")

		userSignup1, err := s.Application.SignupService().Activate(ctx1, event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])
		require.NoError(s.T(), err)
		require.Equal(s.T(), event.Name, userSignup1.Labels[toolchainv1alpha1.UserSignupToolchainEventLabelKey])

		userSignup2, err := s.Application.SignupService().Activate(ctx2, event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])
		require.NoError(s.T(), err)
		require.Equal(s.T(), event.Name, userSignup2.Labels[toolchainv1alpha1.UserSignupToolchainEventLabelKey])

		userSignup3, err := s.Application.SignupService().Activate(ctx3, event.Labels[toolchainv1alpha1.ToolchainEventActivationCodeLabelKey])
		require.Error(s.T(), err)
		require.Equal(s.T(), "limit reached for activation code", err.Error())
		require.Nil(s.T(), userSignup3)
	})
}

func (s *TestSignupServiceSuite) newUserSignupComplete() *toolchainv1alpha1.UserSignup {
	return s.newUserSignupCompleteWithReason("")
}

func (s *TestSignupServiceSuite) newUserSignupCompleteWithReason(reason string) *toolchainv1alpha1.UserSignup {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	return &toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: TestNamespace,
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "ted@domain.com",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "ted@domain.com",
		},
		Status: toolchainv1alpha1.UserSignupStatus{
			Conditions: []toolchainv1alpha1.Condition{
				{
					Type:   toolchainv1alpha1.UserSignupComplete,
					Status: apiv1.ConditionTrue,
					Reason: reason,
				},
				{
					Type:   toolchainv1alpha1.UserSignupApproved,
					Status: apiv1.ConditionTrue,
					Reason: toolchainv1alpha1.UserSignupApprovedAutomaticallyReason,
				},
			},
			CompliantUsername: "ted",
		},
	}
}

func (s *TestSignupServiceSuite) newProvisionedMUR() *toolchainv1alpha1.MasterUserRecord {
	return &toolchainv1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ted",
			Namespace: TestNamespace,
		},
		Spec: toolchainv1alpha1.MasterUserRecordSpec{
			UserAccounts: []toolchainv1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
		},
		Status: toolchainv1alpha1.MasterUserRecordStatus{
			Conditions: []toolchainv1alpha1.Condition{
				{
					Type:    toolchainv1alpha1.MasterUserRecordReady,
					Status:  apiv1.ConditionTrue,
					Reason:  "mur_ready_reason",
					Message: "mur_ready_message",
				},
			},
			UserAccounts: []toolchainv1alpha1.UserAccountStatusEmbedded{{Cluster: toolchainv1alpha1.Cluster{
				Name: "member-123",
			}}},
		},
	}
}

// ToolchainEventOptionIsActive sets the start time of the specified event to 24 hours in the past, the end time to
// 24 hours in the future, and sets the "Ready" condition to true
func ToolchainEventOptionActive(event *toolchainv1alpha1.ToolchainEvent) {
	now := time.Now()

	event.Spec.StartTime = v1.Time{
		Time: now.Add(-24 * time.Hour),
	}
	event.Spec.EndTime = v1.Time{
		Time: now.Add(24 * time.Hour),
	}

	event.Status.Conditions, _ = condition.AddOrUpdateStatusConditions(event.Status.Conditions,
		toolchainv1alpha1.Condition{
			Type:   toolchainv1alpha1.ToolchainEventReady,
			Status: apiv1.ConditionTrue,
		})
}

// ToolchainEventOptionNotActive sets the start time of the specified event to 24 hours in the future, and the
// end time to 48 hours in the future
func ToolchainEventOptionNotActive(event *toolchainv1alpha1.ToolchainEvent) {
	now := time.Now()

	event.Spec.StartTime = v1.Time{
		Time: now.Add(24 * time.Hour),
	}
	event.Spec.EndTime = v1.Time{
		Time: now.Add(48 * time.Hour),
	}
}

// ToolchainEventOptionExpired sets the start time of the specified event to 48 hours in the past, and the
// end time to 24 hours in the past
func ToolchainEventOptionExpired(event *toolchainv1alpha1.ToolchainEvent) {
	now := time.Now()

	event.Spec.StartTime = v1.Time{
		Time: now.Add(-48 * time.Hour),
	}
	event.Spec.EndTime = v1.Time{
		Time: now.Add(-24 * time.Hour),
	}
}

type ToolchainEventOption = func(event *toolchainv1alpha1.ToolchainEvent)

func (s *TestSignupServiceSuite) newToolchainEvent(maxAttendees int, verificationRequired bool, opts ...ToolchainEventOption) (*toolchainv1alpha1.ToolchainEvent, error) {
	eventName := fmt.Sprintf("event-%s", uuid.Must(uuid.NewV4()).String())
	activationCode, err := generateActivationCode()
	if err != nil {
		return nil, err
	}

	event := &toolchainv1alpha1.ToolchainEvent{
		ObjectMeta: v1.ObjectMeta{
			Name:      eventName,
			Namespace: TestNamespace,
			Labels: map[string]string{
				toolchainv1alpha1.ToolchainEventActivationCodeLabelKey: activationCode,
			},
		},
		Spec: toolchainv1alpha1.ToolchainEventSpec{
			MaxAttendees:         maxAttendees,
			VerificationRequired: verificationRequired,
		},
		Status: toolchainv1alpha1.ToolchainEventStatus{
			Conditions: []toolchainv1alpha1.Condition{},
		},
	}

	for _, opt := range opts {
		opt(event)
	}

	return event, nil
}

func generateActivationCode() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	charsetLen := len(activationCodeCharset)
	for i := 0; i < 16; i++ {
		buf[i] = activationCodeCharset[int(buf[i])%charsetLen]
	}

	activationCode := fmt.Sprintf("%s-%s-%s-%s",
		string(buf[0:4]),
		string(buf[4:8]),
		string(buf[8:12]),
		string(buf[12:16]))

	return activationCode, nil
}

func deactivated() []toolchainv1alpha1.Condition {
	return []toolchainv1alpha1.Condition{
		{
			Type:   toolchainv1alpha1.UserSignupComplete,
			Status: apiv1.ConditionTrue,
			Reason: "Deactivated",
		},
	}
}
