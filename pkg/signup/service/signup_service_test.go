package service_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/codeready-toolchain/registration-service/pkg/util"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	errors2 "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/signup/service"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	test2 "github.com/codeready-toolchain/toolchain-common/pkg/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
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
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)
	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	assertUserSignupExists := func(userSignup *toolchainv1alpha1.UserSignup, username string) (schema.GroupVersionResource, toolchainv1alpha1.UserSignup) {
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
		require.Equal(s.T(), username, val.Name)
		require.True(s.T(), states.VerificationRequired(&val))
		require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])
		require.Empty(s.T(), val.Annotations[toolchainv1alpha1.SkipAutoCreateSpaceAnnotationKey]) // skip auto create space annotation is not set by default

		// Confirm all the IdentityClaims have been correctly set
		require.Equal(s.T(), username, val.Spec.IdentityClaims.PreferredUsername)
		require.Equal(s.T(), "jane", val.Spec.IdentityClaims.GivenName)
		require.Equal(s.T(), "doe", val.Spec.IdentityClaims.FamilyName)
		require.Equal(s.T(), "red hat", val.Spec.IdentityClaims.Company)
		require.Equal(s.T(), userID.String(), val.Spec.IdentityClaims.Sub)
		require.Equal(s.T(), "13349822", val.Spec.IdentityClaims.UserID)
		require.Equal(s.T(), "45983711", val.Spec.IdentityClaims.AccountID)
		require.Equal(s.T(), "original-sub-value", val.Spec.IdentityClaims.OriginalSub)
		require.Equal(s.T(), "jsmith@gmail.com", val.Spec.IdentityClaims.Email)

		return gvr, val
	}

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.OriginalSubKey, "original-sub-value")
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	ctx.Set(context.GivenNameKey, "jane")
	ctx.Set(context.FamilyNameKey, "doe")
	ctx.Set(context.CompanyKey, "red hat")
	ctx.Set(context.UserIDKey, "13349822")
	ctx.Set(context.AccountIDKey, "45983711")

	// when
	userSignup, err := s.Application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // at this point, the activation counter annotation is not set
	assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey]) // at this point, the last target cluster annotation is not set
	require.Equal(s.T(), "original-sub-value", userSignup.Spec.IdentityClaims.OriginalSub)

	gvr, existing := assertUserSignupExists(userSignup, "jsmith")

	s.Run("deactivate and reactivate again", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		deactivatedUS.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey] = "2"        // assume the user was activated 2 times already
		deactivatedUS.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey] = "member-3" // assume the user was targeted to member-3
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = deactivated()
		err := s.FakeUserSignupClient.Tracker.Update(gvr, deactivatedUS, configuration.Namespace())
		require.NoError(s.T(), err)

		// when
		deactivatedUS, err = s.Application.SignupService().Signup(ctx)

		// then
		require.NoError(s.T(), err)
		assertUserSignupExists(deactivatedUS, "jsmith")
		assert.NotEmpty(s.T(), deactivatedUS.ResourceVersion)
		assert.Equal(s.T(), "2", deactivatedUS.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey])        // value was preserved
		assert.Equal(s.T(), "member-3", deactivatedUS.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey]) // value was preserved
	})

	s.Run("deactivate and reactivate with missing annotation", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = deactivated()
		// also, alter the activation counter annotation
		delete(deactivatedUS.Annotations, toolchainv1alpha1.UserSignupActivationCounterAnnotationKey)
		delete(deactivatedUS.Annotations, toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey)
		err := s.FakeUserSignupClient.Tracker.Update(gvr, deactivatedUS, configuration.Namespace())
		require.NoError(s.T(), err)

		// when
		userSignup, err := s.Application.SignupService().Signup(ctx)

		// then
		require.NoError(s.T(), err)
		assertUserSignupExists(userSignup, "jsmith")
		assert.NotEmpty(s.T(), userSignup.ResourceVersion)
		assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // was initially missing, and was not set
		assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey]) // was initially missing, and was not set
	})

	s.Run("deactivate and try to reactivate but reactivation fails", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = deactivated()
		err := s.FakeUserSignupClient.Tracker.Update(gvr, deactivatedUS, configuration.Namespace())
		require.NoError(s.T(), err)
		s.FakeUserSignupClient.MockUpdate = func(signup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
			if signup.Name == "jsmith" {
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
func (s *TestSignupServiceSuite) TestSignupFailsWhenClientReturnsError() {

	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "zoeabernathy")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.OriginalSubKey, "original-sub-value")
	ctx.Set(context.EmailKey, "zabernathy@gmail.com")
	ctx.Set(context.GivenNameKey, "zoe")
	ctx.Set(context.FamilyNameKey, "abernathy")
	ctx.Set(context.CompanyKey, "red hat")

	s.FakeUserSignupClient.MockGet = func(_ string) (*toolchainv1alpha1.UserSignup, error) {
		return nil, errors2.NewInternalError(errors.New("an internal error"), "an internal error happened")
	}

	// when
	_, err = s.Application.SignupService().Signup(ctx)
	require.EqualError(s.T(), err, "an internal error: an internal error happened")
}

func (s *TestSignupServiceSuite) TestSignupFailsWithNotFoundThenOtherError() {

	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "lisasmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.OriginalSubKey, "original-sub-value")
	ctx.Set(context.EmailKey, "lsmith@gmail.com")
	ctx.Set(context.GivenNameKey, "lisa")
	ctx.Set(context.FamilyNameKey, "smith")
	ctx.Set(context.CompanyKey, "red hat")

	s.FakeUserSignupClient.MockGet = func(id string) (*toolchainv1alpha1.UserSignup, error) {
		if id == userID.String() {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, id)
		}
		return nil, errors2.NewInternalError(errors.New("something bad happened"), "something very bad happened")
	}

	// when
	_, err = s.Application.SignupService().Signup(ctx)
	require.EqualError(s.T(), err, "something bad happened: something very bad happened")
}

func (s *TestSignupServiceSuite) TestGetSignupFailsWithNotFoundThenOtherError() {

	// given
	s.FakeUserSignupClient.MockGet = func(id string) (*toolchainv1alpha1.UserSignup, error) {
		if id == "000" {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, id)
		}
		return nil, errors2.NewInternalError(errors.New("something quite unfortunate happened"), "something bad")
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// when
	_, err := s.Application.SignupService().GetSignup(c, "000", "abc")

	// then
	require.EqualError(s.T(), err, "something quite unfortunate happened: something bad")

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == "000" {
				return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
			}
			return nil, errors2.NewInternalError(errors.New("something quite unfortunate happened"), "something bad")
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		_, err := svc.GetSignupFromInformer(c, "000", "abc", true)

		// then
		require.EqualError(t, err, "something quite unfortunate happened: something bad")
	})
}

func (s *TestSignupServiceSuite) TestSignupNoSpaces() {
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

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
	ctx.Request, _ = http.NewRequest("POST", "/?no-space=true", bytes.NewBufferString(""))

	// when
	userSignup, err := s.Application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
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
	require.Equal(s.T(), "true", val.Annotations[toolchainv1alpha1.SkipAutoCreateSpaceAnnotationKey]) // skip auto create space annotation is set
}

func (s *TestSignupServiceSuite) TestSignupWithCaptchaEnabled() {
	test2.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, configuration.Namespace())

	// captcha is enabled
	serviceOption := func(svc *service.ServiceImpl) {
		svc.CaptchaChecker = FakeCaptchaChecker{score: 0.9} // score is above threshold
	}

	opt := func(serviceFactory *factory.ServiceFactory) {
		serviceFactory.WithSignupServiceOption(serviceOption)
	}

	s.WithFactoryOption(opt)

	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().Enabled(true).
			Verification().CaptchaEnabled(true).
			Verification().CaptchaScoreThreshold("0.8"))

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
	ctx.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString(""))
	ctx.Request.Header.Set("Recaptcha-Token", "abc")

	// when
	userSignup, err := s.Application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
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
	require.Equal(s.T(), "0.9", val.Annotations[toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey]) // captcha score annotation is set
}

func (s *TestSignupServiceSuite) TestUserSignupWithInvalidSubjectPrefix() {
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	username := "-sjones"

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, username)
	ctx.Set(context.SubKey, userID.String())
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
	expected := fmt.Sprintf("%x%s", crc32.Checksum([]byte(username), crc32.IEEETable), username)

	require.Equal(s.T(), expected, val.Name)
	require.False(s.T(), strings.HasPrefix(val.Name, "-"))
}

func (s *TestSignupServiceSuite) TestEncodeUserID() {
	s.Run("test valid user ID unchanged", func() {
		userID := "abcde-12345"
		encoded := service.EncodeUserIdentifier(userID)
		require.Equal(s.T(), userID, encoded)
	})
	s.Run("test user ID with invalid characters", func() {
		userID := "abcde\\*-12345"
		encoded := service.EncodeUserIdentifier(userID)
		require.Equal(s.T(), "c0177ca4-abcde-12345", encoded)
	})
	s.Run("test user ID with invalid prefix", func() {
		userID := "-1234567"
		encoded := service.EncodeUserIdentifier(userID)
		require.Equal(s.T(), "ca3e1e0f-1234567", encoded)
	})
	s.Run("test user ID that exceeds max length", func() {
		userID := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-01234567890123456789"
		encoded := service.EncodeUserIdentifier(userID)
		require.Equal(s.T(), "e3632025-0123456789abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr", encoded)
	})
	s.Run("test user ID with colon separator", func() {
		userID := "abc:xyz"
		encoded := service.EncodeUserIdentifier(userID)
		require.Equal(s.T(), "a05a4053-abcxyz", encoded)
	})
	s.Run("test user ID with invalid end character", func() {
		userID := "abc---"
		encoded := service.EncodeUserIdentifier(userID)
		require.Equal(s.T(), "ed6bd2b5-abc", encoded)
	})
}

func (s *TestSignupServiceSuite) TestUserWithExcludedDomainEmailSignsUp() {
	s.ServiceConfiguration(configuration.Namespace(), true, "redhat.com", 5)

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

	values, err := s.FakeUserSignupClient.Tracker.List(gvr, gvk, configuration.Namespace())
	require.NoError(s.T(), err)

	userSignups := values.(*toolchainv1alpha1.UserSignupList)
	require.NotEmpty(s.T(), userSignups.Items)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.False(s.T(), states.VerificationRequired(&val))
}

func (s *TestSignupServiceSuite) TestCRTAdminUserSignup() {
	s.ServiceConfiguration(configuration.Namespace(), true, "redhat.com", 5)

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
	require.EqualError(s.T(), err, "forbidden: failed to create usersignup for jsmith-crtadmin")
	require.Nil(s.T(), userSignup)
}

func (s *TestSignupServiceSuite) TestFailsIfUserSignupNameAlreadyExists() {
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	err = s.FakeUserSignupClient.Tracker.Add(&toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: configuration.Namespace(),
		},
		Spec: toolchainv1alpha1.UserSignupSpec{},
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
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = s.FakeBannedUserClient.Tracker.Add(&toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: configuration.Namespace(),
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
	e := &apierrors.StatusError{}
	require.ErrorAs(s.T(), err, &e)
	require.Equal(s.T(), "Failure", e.ErrStatus.Status)
	require.Equal(s.T(), "forbidden: user has been banned", e.ErrStatus.Message)
	require.Equal(s.T(), v1.StatusReasonForbidden, e.ErrStatus.Reason)
}

func (s *TestSignupServiceSuite) TestPhoneNumberAlreadyInUseBannedUser() {
	s.ServiceConfiguration(configuration.Namespace(), true, "redhat.com", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = s.FakeBannedUserClient.Tracker.Add(&toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: configuration.Namespace(),
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
	err = s.Application.SignupService().PhoneNumberAlreadyInUse(bannedUserID.String(), "jsmith", "+12268213044")
	require.EqualError(s.T(), err, "cannot re-register with phone number: phone number already in use")
}

func (s *TestSignupServiceSuite) TestPhoneNumberAlreadyInUseUserSignup() {
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = s.FakeUserSignupClient.Tracker.Add(&toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: configuration.Namespace(),
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
	err = s.Application.SignupService().PhoneNumberAlreadyInUse(newUserID.String(), "jsmith", "+12268213044")
	require.EqualError(s.T(), err, "cannot re-register with phone number: phone number already in use")
}

func (s *TestSignupServiceSuite) TestOKIfOtherUserBanned() {
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	err = s.FakeBannedUserClient.Tracker.Add(&toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: configuration.Namespace(),
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

	values, err := s.FakeUserSignupClient.Tracker.List(gvr, gvk, configuration.Namespace())
	require.NoError(s.T(), err)

	userSignups := values.(*toolchainv1alpha1.UserSignupList)
	require.NotEmpty(s.T(), userSignups.Items)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.Equal(s.T(), configuration.Namespace(), val.Namespace)
	require.Equal(s.T(), "jsmith", val.Name)
	require.False(s.T(), states.ApprovedManually(&val))
	require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])
}

func (s *TestSignupServiceSuite) TestGetUserSignupFails() {
	// given
	username := "johnsmith"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	s.FakeUserSignupClient.MockGet = func(name string) (*toolchainv1alpha1.UserSignup, error) {
		if name == username {
			return nil, errors.New("an error occurred")
		}
		return &toolchainv1alpha1.UserSignup{}, nil
	}

	// when
	_, err := s.Application.SignupService().GetSignup(c, "", username)

	// then
	require.EqualError(s.T(), err, "an error occurred")

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == username {
				return nil, errors.New("an error occurred")
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		_, err := svc.GetSignupFromInformer(c, "johnsmith", "abc", true)

		// then
		require.EqualError(t, err, "an error occurred")
	})
}

func (s *TestSignupServiceSuite) TestGetSignupNotFound() {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	signup, err := s.Application.SignupService().GetSignup(c, userID.String(), "")
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()

		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		signup, err := svc.GetSignupFromInformer(c, userID.String(), "", true)

		// then
		require.Nil(t, signup)
		require.NoError(t, err)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupStatusNotComplete() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	userSignupNotComplete := toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: configuration.Namespace(),
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "bill",
			},
		},
		Status: toolchainv1alpha1.UserSignupStatus{
			CompliantUsername: "bill",
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
	states.SetVerificationRequired(&userSignupNotComplete, true)

	err = s.FakeUserSignupClient.Tracker.Add(&userSignupNotComplete)
	require.NoError(s.T(), err)

	// when
	response, err := s.Application.SignupService().GetSignup(c, userID.String(), "")

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), userID.String(), response.Name)
	require.Equal(s.T(), "bill", response.Username)
	require.Equal(s.T(), "bill", response.CompliantUsername)
	require.False(s.T(), response.Status.Ready)
	require.Equal(s.T(), "test_reason", response.Status.Reason)
	require.Equal(s.T(), "test_message", response.Status.Message)
	require.True(s.T(), response.Status.VerificationRequired)
	require.Empty(s.T(), response.ConsoleURL)
	require.Empty(s.T(), response.CheDashboardURL)
	require.Empty(s.T(), response.APIEndpoint)
	require.Empty(s.T(), response.ClusterName)
	require.Empty(s.T(), response.ProxyURL)
	assert.Empty(s.T(), response.DefaultUserNamespace)
	assert.Empty(s.T(), response.RHODSMemberURL)

	s.T().Run("informer - with check for usersignup complete condition", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == userID.String() {
				return &userSignupNotComplete, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		response, err := svc.GetSignupFromInformer(c, userID.String(), "", true)

		// then
		require.NoError(t, err)
		require.NotNil(t, response)

		require.Equal(t, userID.String(), response.Name)
		require.Equal(t, "bill", response.Username)
		require.Equal(t, "bill", response.CompliantUsername)
		require.False(t, response.Status.Ready)
		require.Equal(t, "test_reason", response.Status.Reason)
		require.Equal(t, "test_message", response.Status.Message)
		require.True(t, response.Status.VerificationRequired)
		require.Empty(t, response.ConsoleURL)
		require.Empty(t, response.CheDashboardURL)
		require.Empty(t, response.APIEndpoint)
		require.Empty(t, response.ClusterName)
		require.Empty(t, response.ProxyURL)
		assert.Equal(t, "", response.DefaultUserNamespace)
		assert.Equal(t, "", response.RHODSMemberURL)
	})

	s.T().Run("informer - with no check for UserSignup complete condition", func(t *testing.T) {
		// given
		states.SetVerificationRequired(&userSignupNotComplete, false)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)
		mur := s.newProvisionedMUR("bill")
		err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
		require.NoError(t, err)

		space := s.newSpaceForMUR(mur.Name, userSignupNotComplete.Name)
		err = s.FakeSpaceClient.Tracker.Add(space)
		require.NoError(t, err)

		spacebinding := s.newSpaceBinding(mur.Name, space.Name)
		err = s.FakeSpaceBindingClient.Tracker.Add(spacebinding)
		require.NoError(t, err)

		toolchainStatus := s.newToolchainStatus(".apps.")
		err = s.FakeToolchainStatusClient.Tracker.Add(toolchainStatus)
		require.NoError(t, err)

		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == userID.String() {
				return &userSignupNotComplete, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		inf.GetMurFunc = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
			if name == mur.Name {
				return mur, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		inf.GetSpaceFunc = func(_ string) (*toolchainv1alpha1.Space, error) {
			return space, nil
		}
		inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			return []toolchainv1alpha1.SpaceBinding{*spacebinding}, nil
		}
		inf.GetToolchainStatusFunc = func() (*toolchainv1alpha1.ToolchainStatus, error) {
			return toolchainStatus, nil
		}
		s.Application.MockInformerService(inf)

		// when
		// we set checkUserSignupCompleted to false
		response, err := svc.GetSignupFromInformer(c, userID.String(), userSignupNotComplete.Spec.IdentityClaims.PreferredUsername, false)

		// then
		require.NoError(t, err)
		require.NotNil(t, response)

		require.Equal(t, userID.String(), response.Name)
		require.Equal(t, "bill", response.Username)
		require.Equal(t, "bill", response.CompliantUsername)
		require.True(t, response.Status.Ready)
		require.Equal(t, "mur_ready_reason", response.Status.Reason)
		require.Equal(t, "mur_ready_message", response.Status.Message)
		require.False(t, response.Status.VerificationRequired)
		require.Equal(t, "https://console.apps.member-123.com", response.ConsoleURL)
		require.Equal(t, "http://che-toolchain-che.member-123.com", response.CheDashboardURL)
		require.Equal(t, "http://api.devcluster.openshift.com", response.APIEndpoint)
		require.Equal(t, "member-123", response.ClusterName)
		require.Equal(t, "https://proxy-url.com", response.ProxyURL)
		assert.Equal(t, "bill-dev", response.DefaultUserNamespace)
		assert.Equal(t, "https://rhods-dashboard-redhat-ods-applications.apps.member-123.com", response.RHODSMemberURL)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupNoStatusNotCompleteCondition() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

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

		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		userSignup := toolchainv1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      userID.String(),
				Namespace: configuration.Namespace(),
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "bill",
				},
			},
			Status: status,
		}

		states.SetVerificationRequired(&userSignup, true)

		err = s.FakeUserSignupClient.Tracker.Add(&userSignup)
		require.NoError(s.T(), err)

		// when
		response, err := s.Application.SignupService().GetSignup(c, userID.String(), "bill")

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), response)

		require.Equal(s.T(), userID.String(), response.Name)
		require.Equal(s.T(), "bill", response.Username)
		require.Empty(s.T(), response.CompliantUsername)
		require.False(s.T(), response.Status.Ready)
		require.Equal(s.T(), "PendingApproval", response.Status.Reason)
		require.True(s.T(), response.Status.VerificationRequired)
		require.Empty(s.T(), response.Status.Message)
		require.Empty(s.T(), response.ConsoleURL)
		require.Empty(s.T(), response.CheDashboardURL)
		require.Empty(s.T(), response.APIEndpoint)
		require.Empty(s.T(), response.ClusterName)
		require.Empty(s.T(), response.ProxyURL)
		assert.Equal(s.T(), "", response.DefaultUserNamespace)
		assert.Equal(s.T(), "", response.RHODSMemberURL)

		s.T().Run("informer", func(t *testing.T) {
			// given
			inf := fake.NewFakeInformer()
			inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
				if name == userID.String() {
					return &userSignup, nil
				}
				return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
			}

			s.Application.MockInformerService(inf)
			svc := service.NewSignupService(
				fake.MemberClusterServiceContext{
					Client: s,
					Svcs:   s.Application,
				},
			)

			// when
			response, err := svc.GetSignupFromInformer(c, userID.String(), "", true)

			// then
			require.NoError(t, err)
			require.NotNil(t, response)

			require.Equal(t, userID.String(), response.Name)
			require.Equal(t, "bill", response.Username)
			require.Empty(t, response.CompliantUsername)
			require.False(t, response.Status.Ready)
			require.Equal(t, "PendingApproval", response.Status.Reason)
			require.True(t, response.Status.VerificationRequired)
			require.Empty(t, response.Status.Message)
			require.Empty(t, response.ConsoleURL)
			require.Empty(t, response.CheDashboardURL)
			require.Empty(t, response.APIEndpoint)
			require.Empty(t, response.ClusterName)
			require.Empty(t, response.ProxyURL)
			assert.Equal(t, "", response.DefaultUserNamespace)
			assert.Equal(t, "", response.RHODSMemberURL)
		})
	}
}

func (s *TestSignupServiceSuite) TestGetSignupDeactivated() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	us := s.newUserSignupComplete()
	us.Status.Conditions = deactivated()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// when
	signup, err := s.Application.SignupService().GetSignup(c, us.Name, "")

	// then
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == us.Name {
				return us, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		signup, err := svc.GetSignupFromInformer(c, us.Name, "", true)

		// then
		require.Nil(t, signup)
		require.NoError(t, err)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupStatusOK() {
	// given
	for _, appsSubDomain := range []string{".apps.", ".apps-"} {
		s.SetupTest()
		s.T().Run("for apps subdomain: "+appsSubDomain, func(t *testing.T) {
			s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

			us := s.newUserSignupComplete()
			err := s.FakeUserSignupClient.Tracker.Add(us)
			require.NoError(t, err)

			mur := s.newProvisionedMUR("ted")
			err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
			require.NoError(t, err)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			toolchainStatus := s.newToolchainStatus(appsSubDomain)
			err = s.FakeToolchainStatusClient.Tracker.Add(toolchainStatus)
			require.NoError(t, err)

			space := s.newSpaceForMUR(mur.Name, us.Name)
			err = s.FakeSpaceClient.Tracker.Add(space)
			require.NoError(t, err)

			spacebinding := s.newSpaceBinding(mur.Name, space.Name)
			err = s.FakeSpaceBindingClient.Tracker.Add(spacebinding)
			require.NoError(t, err)

			// when
			response, err := s.Application.SignupService().GetSignup(c, us.Name, "")

			// then
			require.NoError(t, err)
			require.NotNil(t, response)

			require.Equal(t, us.Name, response.Name)
			require.Equal(t, "jsmith", response.Username)
			require.Equal(t, "ted", response.CompliantUsername)
			require.NotNil(t, response.DaysRemaining)
			require.InEpsilon(t, float64(30), *response.DaysRemaining, 0.1)
			require.Equal(t, mur.Status.ProvisionedTime.Format(time.RFC3339), response.StartDate)
			require.Equal(t, us.Status.ScheduledDeactivationTimestamp.Format(time.RFC3339), response.EndDate)
			assert.True(t, response.Status.Ready)
			assert.Equal(t, "mur_ready_reason", response.Status.Reason)
			assert.Equal(t, "mur_ready_message", response.Status.Message)
			assert.False(t, response.Status.VerificationRequired)
			assert.Equal(t, fmt.Sprintf("https://console%smember-123.com", appsSubDomain), response.ConsoleURL)
			assert.Equal(t, "http://che-toolchain-che.member-123.com", response.CheDashboardURL)
			assert.Equal(t, "http://api.devcluster.openshift.com", response.APIEndpoint)
			assert.Equal(t, "member-123", response.ClusterName)
			assert.Equal(t, "https://proxy-url.com", response.ProxyURL)
			assert.Equal(t, "ted-dev", response.DefaultUserNamespace)
			assert.Equal(t, fmt.Sprintf("https://rhods-dashboard-redhat-ods-applications%smember-123.com", appsSubDomain), response.RHODSMemberURL)

			s.T().Run("informer", func(t *testing.T) {
				// given
				inf := fake.NewFakeInformer()
				inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
					if name == us.Name {
						return us, nil
					}
					return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
				}
				inf.GetMurFunc = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
					if name == mur.Name {
						return mur, nil
					}
					return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
				}
				inf.GetToolchainStatusFunc = func() (*toolchainv1alpha1.ToolchainStatus, error) {
					return toolchainStatus, nil
				}
				inf.GetSpaceFunc = func(_ string) (*toolchainv1alpha1.Space, error) {
					return space, nil
				}
				inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
					return []toolchainv1alpha1.SpaceBinding{*spacebinding}, nil
				}

				s.Application.MockInformerService(inf)
				svc := service.NewSignupService(
					fake.MemberClusterServiceContext{
						Client: s,
						Svcs:   s.Application,
					},
				)

				// when
				response, err := svc.GetSignupFromInformer(c, us.Name, "", true)

				// then
				require.NoError(t, err)
				require.NotNil(t, response)

				require.Equal(t, "jsmith", response.Username)
				require.Equal(t, "ted", response.CompliantUsername)
				assert.True(t, response.Status.Ready)
				assert.Equal(t, "mur_ready_reason", response.Status.Reason)
				assert.Equal(t, "mur_ready_message", response.Status.Message)
				assert.False(t, response.Status.VerificationRequired)
				assert.Equal(t, fmt.Sprintf("https://console%smember-123.com", appsSubDomain), response.ConsoleURL)
				assert.Equal(t, "http://che-toolchain-che.member-123.com", response.CheDashboardURL)
				assert.Equal(t, "http://api.devcluster.openshift.com", response.APIEndpoint)
				assert.Equal(t, "member-123", response.ClusterName)
				assert.Equal(t, "https://proxy-url.com", response.ProxyURL)
				assert.Equal(t, "ted-dev", response.DefaultUserNamespace)
				assert.Equal(t, fmt.Sprintf("https://rhods-dashboard-redhat-ods-applications%smember-123.com", appsSubDomain), response.RHODSMemberURL)
			})
		})
	}
}

func (s *TestSignupServiceSuite) TestGetSignupByUsernameOK() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	us := s.newUserSignupComplete()
	us.Name = service.EncodeUserIdentifier(us.Spec.IdentityClaims.PreferredUsername)
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	mur := s.newProvisionedMUR("ted")
	err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
	require.NoError(s.T(), err)

	svc := service.NewSignupService(
		fake.MemberClusterServiceContext{
			Client: s,
			Svcs:   s.Application,
		},
	)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	space := s.newSpaceForMUR(mur.Name, us.Name)
	err = s.FakeSpaceClient.Tracker.Add(space)
	require.NoError(s.T(), err)

	spacebinding := s.newSpaceBinding(mur.Name, space.Name)
	err = s.FakeSpaceBindingClient.Tracker.Add(spacebinding)
	require.NoError(s.T(), err)

	toolchainStatus := s.newToolchainStatus(".apps.")
	err = s.FakeToolchainStatusClient.Tracker.Add(toolchainStatus)
	require.NoError(s.T(), err)

	// when
	response, err := svc.GetSignup(c, "foo", us.Spec.IdentityClaims.PreferredUsername)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), us.Name, response.Name)
	require.Equal(s.T(), "jsmith", response.Username)
	require.Equal(s.T(), "ted", response.CompliantUsername)
	assert.True(s.T(), response.Status.Ready)
	assert.Equal(s.T(), "mur_ready_reason", response.Status.Reason)
	assert.Equal(s.T(), "mur_ready_message", response.Status.Message)
	assert.False(s.T(), response.Status.VerificationRequired)
	assert.Equal(s.T(), "https://console.apps.member-123.com", response.ConsoleURL)
	assert.Equal(s.T(), "http://che-toolchain-che.member-123.com", response.CheDashboardURL)
	assert.Equal(s.T(), "http://api.devcluster.openshift.com", response.APIEndpoint)
	assert.Equal(s.T(), "member-123", response.ClusterName)
	assert.Equal(s.T(), "https://proxy-url.com", response.ProxyURL)
	assert.Equal(s.T(), "ted-dev", response.DefaultUserNamespace)
	assert.Equal(s.T(), "https://rhods-dashboard-redhat-ods-applications.apps.member-123.com", response.RHODSMemberURL)

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == us.Name {
				return us, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		inf.GetMurFunc = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
			if name == mur.Name {
				return mur, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		inf.GetToolchainStatusFunc = func() (*toolchainv1alpha1.ToolchainStatus, error) {
			return toolchainStatus, nil
		}
		inf.GetSpaceFunc = func(_ string) (*toolchainv1alpha1.Space, error) {
			return space, nil
		}
		inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			return []toolchainv1alpha1.SpaceBinding{*spacebinding}, nil
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		response, err := svc.GetSignupFromInformer(c, "foo", us.Spec.IdentityClaims.PreferredUsername, true)

		// then
		require.NoError(t, err)
		require.NotNil(t, response)

		require.Equal(t, us.Name, response.Name)
		require.Equal(t, "jsmith", response.Username)
		require.Equal(t, "ted", response.CompliantUsername)
		assert.True(t, response.Status.Ready)
		assert.Equal(t, "mur_ready_reason", response.Status.Reason)
		assert.Equal(t, "mur_ready_message", response.Status.Message)
		assert.False(t, response.Status.VerificationRequired)
		assert.Equal(t, "https://console.apps.member-123.com", response.ConsoleURL)
		assert.Equal(t, "http://che-toolchain-che.member-123.com", response.CheDashboardURL)
		assert.Equal(t, "http://api.devcluster.openshift.com", response.APIEndpoint)
		assert.Equal(t, "member-123", response.ClusterName)
		assert.Equal(t, "https://proxy-url.com", response.ProxyURL)
		assert.Equal(t, "ted-dev", response.DefaultUserNamespace)
		assert.Equal(t, "https://rhods-dashboard-redhat-ods-applications.apps.member-123.com", response.RHODSMemberURL)
	})
}

func (s *TestSignupServiceSuite) newToolchainStatus(appsSubDomain string) *toolchainv1alpha1.ToolchainStatus {
	toolchainStatus := &toolchainv1alpha1.ToolchainStatus{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "toolchain-status",
			Namespace: configuration.Namespace(),
		},
		Status: toolchainv1alpha1.ToolchainStatusStatus{
			Members: []toolchainv1alpha1.Member{
				{
					ClusterName: "member-1",
					APIEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL:      fmt.Sprintf("https://console%smember-1.com", appsSubDomain),
							CheDashboardURL: "http://che-toolchain-che.member-1.com",
						},
					},
				},
				{
					ClusterName: "member-123",
					APIEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL:      fmt.Sprintf("https://console%smember-123.com", appsSubDomain),
							CheDashboardURL: "http://che-toolchain-che.member-123.com",
						},
					},
				},
			},
			HostRoutes: toolchainv1alpha1.HostRoutes{
				ProxyURL: "https://proxy-url.com",
			},
		},
	}
	return toolchainStatus
}

func (s *TestSignupServiceSuite) TestGetSignupStatusFailGetToolchainStatus() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	mur := s.newProvisionedMUR("ted")
	err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
	require.NoError(s.T(), err)

	// when
	_, err = s.Application.SignupService().GetSignup(c, us.Name, "")

	// then
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s: toolchainstatuses.toolchain.dev.openshift.com \"toolchain-status\" not found", us.Name))

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == us.Name {
				return us, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		inf.GetMurFunc = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
			if name == mur.Name {
				return mur, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		inf.GetToolchainStatusFunc = func() (*toolchainv1alpha1.ToolchainStatus, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, "toolchain-status")
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		_, err := svc.GetSignupFromInformer(c, us.Name, "", true)

		// then
		require.EqualError(t, err, fmt.Sprintf("error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s:  \"toolchain-status\" not found", us.Name))
	})
}

func (s *TestSignupServiceSuite) TestGetSignupMURGetFails() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	returnedErr := errors.New("an error occurred")
	s.FakeMasterUserRecordClient.MockGet = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
		if name == us.Status.CompliantUsername {
			return nil, returnedErr
		}
		return &toolchainv1alpha1.MasterUserRecord{}, nil
	}

	// when
	_, err = s.Application.SignupService().GetSignup(c, us.Name, "")

	// then
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", us.Name))

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == us.Name {
				return us, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		inf.GetMurFunc = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
			if name == us.Status.CompliantUsername {
				return nil, returnedErr
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		_, err := svc.GetSignupFromInformer(c, us.Name, "", true)

		// then
		require.EqualError(t, err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", us.Name))
	})
}

func (s *TestSignupServiceSuite) TestGetSignupReadyConditionStatus() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	mur := &toolchainv1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ted",
			Namespace: configuration.Namespace(),
		},
		Spec: toolchainv1alpha1.MasterUserRecordSpec{
			UserAccounts: []toolchainv1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
		},
	}

	tests := map[string]struct {
		condition              toolchainv1alpha1.Condition
		expectedConditionReady bool
	}{
		"no ready condition": {
			condition:              toolchainv1alpha1.Condition{},
			expectedConditionReady: false,
		},
		"ready condition without status": {
			condition: toolchainv1alpha1.Condition{
				Type:    toolchainv1alpha1.ConditionReady,
				Reason:  "some reason",
				Message: "some message",
			},
			expectedConditionReady: false,
		},
		"ready condition with unknown status": {
			condition: toolchainv1alpha1.Condition{
				Type:    toolchainv1alpha1.ConditionReady,
				Reason:  "some reason",
				Message: "some message",
				Status:  apiv1.ConditionUnknown,
			},
			expectedConditionReady: false,
		},
		"ready condition with false status": {
			condition: toolchainv1alpha1.Condition{
				Type:    toolchainv1alpha1.ConditionReady,
				Reason:  "some reason",
				Message: "some message",
				Status:  apiv1.ConditionFalse,
			},
			expectedConditionReady: false,
		},
		"ready condition with true status": {
			condition: toolchainv1alpha1.Condition{
				Type:    toolchainv1alpha1.ConditionReady,
				Reason:  "some reason",
				Message: "some message",
				Status:  apiv1.ConditionTrue,
			},
			expectedConditionReady: true,
		},
	}

	for tcName, tc := range tests {
		s.T().Run(tcName, func(t *testing.T) {

			// given
			mur.Status = toolchainv1alpha1.MasterUserRecordStatus{
				Conditions: []toolchainv1alpha1.Condition{
					tc.condition,
				},
			}
			err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
			require.NoError(t, err)

			// when
			response, err := s.Application.SignupService().GetSignup(c, us.Name, "")

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedConditionReady, response.Status.Ready)
			require.Equal(t, tc.condition.Reason, response.Status.Reason)
			require.Equal(t, tc.condition.Message, response.Status.Message)

			// informer case
			// given
			inf := fake.NewFakeInformer()
			inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
				if name == us.Name {
					return us, nil
				}
				return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
			}
			inf.GetMurFunc = func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
				if name == mur.Name {
					return mur, nil
				}
				return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
			}

			s.Application.MockInformerService(inf)
			svc := service.NewSignupService(
				fake.MemberClusterServiceContext{
					Client: s,
					Svcs:   s.Application,
				},
			)

			// when
			_, err = svc.GetSignupFromInformer(c, us.Name, "", true)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedConditionReady, response.Status.Ready)
			require.Equal(t, tc.condition.Reason, response.Status.Reason)
			require.Equal(t, tc.condition.Message, response.Status.Message)
			err = s.FakeMasterUserRecordClient.Delete(mur.Name, nil)
			require.NoError(t, err)
		})
	}
}

func (s *TestSignupServiceSuite) TestGetSignupBannedUserEmail() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	us := s.newBannedUserSignup()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, us.Spec.IdentityClaims.UserID)
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	// when
	response, err := s.Application.SignupService().GetSignup(ctx, us.Name, "")

	// then
	// return not found signup
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)
	require.Equal(s.T(), toolchainv1alpha1.UserSignupPendingApprovalReason, response.Status.Reason)

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetUserSignupFunc = func(name string) (*toolchainv1alpha1.UserSignup, error) {
			if name == us.Name {
				return us, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		response, err := svc.GetSignupFromInformer(ctx, us.Name, "", true)

		// then
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, toolchainv1alpha1.UserSignupPendingApprovalReason, response.Status.Reason)

	})
}

func (s *TestSignupServiceSuite) TestGetDefaultUserNamespace() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	signup := signup.Signup{
		Name:              "dave#123",
		CompliantUsername: "dave",
	}

	space := s.newSpaceForMUR(signup.CompliantUsername, signup.Name)
	err := s.FakeSpaceClient.Tracker.Add(space)
	require.NoError(s.T(), err)

	spacebinding := s.newSpaceBinding(signup.CompliantUsername, space.Name)
	err = s.FakeSpaceBindingClient.Tracker.Add(spacebinding)
	require.NoError(s.T(), err)

	// when
	defaultUserNamespace := service.GetDefaultUserNamespace(s, signup)

	// then
	assert.Equal(s.T(), "dave-dev", defaultUserNamespace)

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetSpaceFunc = func(_ string) (*toolchainv1alpha1.Space, error) {
			return space, nil
		}
		inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			return []toolchainv1alpha1.SpaceBinding{*spacebinding}, nil
		}

		// when
		defaultUserNamespace := service.GetDefaultUserNamespace(inf, signup)

		// then
		assert.Equal(t, "dave-dev", defaultUserNamespace)
	})
}

// TestGetDefaultUserNamespaceOnlyUnownedSpace tests that the default user namespace is returned even if the only accessible Space was not created by the user.
// This is valuable because if the creator label on a Space is missing for whatever reason, APIs that depend on this field will still work.
func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceOnlyUnownedSpace() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	signupA := signup.Signup{
		Name:              "userA#123",
		CompliantUsername: "userA",
	}

	signupB := signup.Signup{
		Name:              "userB#123",
		CompliantUsername: "userB",
	}

	// space created by userA
	space := s.newSpaceForMUR(signupA.CompliantUsername, signupA.Name)
	err := s.FakeSpaceClient.Tracker.Add(space)
	require.NoError(s.T(), err)

	// space shared with userB
	spacebinding := s.newSpaceBinding(signupB.CompliantUsername, space.Name)
	err = s.FakeSpaceBindingClient.Tracker.Add(spacebinding)
	require.NoError(s.T(), err)

	// when
	defaultUserNamespace := service.GetDefaultUserNamespace(s, signupB)

	// then
	assert.Equal(s.T(), "userA-dev", defaultUserNamespace)

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetSpaceFunc = func(_ string) (*toolchainv1alpha1.Space, error) {
			return space, nil
		}
		inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			return []toolchainv1alpha1.SpaceBinding{*spacebinding}, nil
		}

		// when
		defaultUserNamespace := service.GetDefaultUserNamespace(inf, signupB)

		// then
		assert.Equal(t, "userA-dev", defaultUserNamespace)
	})
}

// TestGetDefaultUserNamespaceMultiSpace tests that the Space created by the user is prioritized when there are multiple spaces
func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceMultiSpace() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	signupA := signup.Signup{
		Name:              "userA#123",
		CompliantUsername: "userA",
	}

	signupB := signup.Signup{
		Name:              "userB#123",
		CompliantUsername: "userB",
	}

	// space1 created by userA
	space1 := s.newSpaceForMUR(signupA.CompliantUsername, signupA.Name)
	err := s.FakeSpaceClient.Tracker.Add(space1)
	require.NoError(s.T(), err)

	// space1 shared with userB
	spacebinding1 := s.newSpaceBinding(signupB.CompliantUsername, space1.Name)
	err = s.FakeSpaceBindingClient.Tracker.Add(spacebinding1)
	require.NoError(s.T(), err)

	// space2 created by userB
	space2 := s.newSpaceForMUR(signupB.CompliantUsername, signupB.Name)
	err = s.FakeSpaceClient.Tracker.Add(space2)
	require.NoError(s.T(), err)

	// space2 shared with userB
	spacebinding2 := s.newSpaceBinding(signupB.CompliantUsername, space2.Name)
	err = s.FakeSpaceBindingClient.Tracker.Add(spacebinding2)
	require.NoError(s.T(), err)

	// when
	// get default namespace for userB
	defaultUserNamespace := service.GetDefaultUserNamespace(s, signupB)

	// then
	assert.Equal(s.T(), "userB-dev", defaultUserNamespace) // space2 is prioritized over space1 because it was created by the userB

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
			switch name {
			case space1.Name:
				return space1, nil
			case space2.Name:
				return space2, nil
			default:
				return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
			}
		}
		inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			return []toolchainv1alpha1.SpaceBinding{*spacebinding1, *spacebinding2}, nil
		}

		// when
		defaultUserNamespace := service.GetDefaultUserNamespace(inf, signupB)

		// then
		assert.Equal(t, "userB-dev", defaultUserNamespace)
	})
}

func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceFailNoSpaceBinding() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	signup := signup.Signup{
		Name:              "dave#123",
		CompliantUsername: "dave",
	}

	space := s.newSpaceForMUR(signup.CompliantUsername, signup.Name)
	err := s.FakeSpaceClient.Tracker.Add(space)
	require.NoError(s.T(), err)

	// when
	defaultUserNamespace := service.GetDefaultUserNamespace(s, signup)

	// then
	assert.Empty(s.T(), defaultUserNamespace)

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetSpaceFunc = func(_ string) (*toolchainv1alpha1.Space, error) {
			return space, nil
		}
		inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			return nil, apierrors.NewInternalError(fmt.Errorf("something went wrong"))
		}

		// when
		defaultUserNamespace := service.GetDefaultUserNamespace(inf, signup)

		// then
		assert.Empty(t, defaultUserNamespace)
	})
}

func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceFailNoSpace() {
	// given
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	signup := signup.Signup{
		Name:              "dave",
		CompliantUsername: "dave",
	}

	spacebinding := s.newSpaceBinding(signup.CompliantUsername, signup.Name)
	err := s.FakeSpaceBindingClient.Tracker.Add(spacebinding)
	require.NoError(s.T(), err)

	// when
	defaultUserNamespace := service.GetDefaultUserNamespace(s, signup)

	// then
	assert.Empty(s.T(), defaultUserNamespace)

	s.T().Run("informer", func(t *testing.T) {
		// given
		inf := fake.NewFakeInformer()
		inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
		inf.ListSpaceBindingFunc = func(_ ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
			return []toolchainv1alpha1.SpaceBinding{*spacebinding}, nil
		}

		// when
		defaultUserNamespace := service.GetDefaultUserNamespace(inf, signup)

		// then
		assert.Empty(t, defaultUserNamespace)
	})
}

func (s *TestSignupServiceSuite) TestGetUserSignup() {
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	s.Run("getusersignup ok", func() {
		us := s.newUserSignupComplete()
		err := s.FakeUserSignupClient.Tracker.Add(us)
		require.NoError(s.T(), err)

		val, err := s.Application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
		require.NoError(s.T(), err)
		require.Equal(s.T(), us.Name, val.Name)
	})

	s.Run("getusersignup returns error", func() {
		s.FakeUserSignupClient.MockGet = func(_ string) (userSignup *toolchainv1alpha1.UserSignup, e error) {
			return nil, errors.New("get failed")
		}

		val, err := s.Application.SignupService().GetUserSignupFromIdentifier("foo", "")
		require.EqualError(s.T(), err, "get failed")
		require.Nil(s.T(), val)
	})

	s.Run("getusersignup with unknown user", func() {
		s.FakeUserSignupClient.MockGet = nil

		val, err := s.Application.SignupService().GetUserSignupFromIdentifier("unknown", "")
		require.True(s.T(), apierrors.IsNotFound(err))
		require.Nil(s.T(), val)
	})
}

func (s *TestSignupServiceSuite) TestUpdateUserSignup() {
	s.ServiceConfiguration(configuration.Namespace(), true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	s.Run("updateusersignup ok", func() {
		val, err := s.Application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
		require.NoError(s.T(), err)

		val.Spec.IdentityClaims.FamilyName = "Johnson"

		updated, err := s.Application.SignupService().UpdateUserSignup(val)
		require.NoError(s.T(), err)

		require.Equal(s.T(), val.Spec.IdentityClaims.FamilyName, updated.Spec.IdentityClaims.FamilyName)
	})

	s.Run("updateusersignup returns error", func() {
		s.FakeUserSignupClient.MockUpdate = func(_ *toolchainv1alpha1.UserSignup) (userSignup *toolchainv1alpha1.UserSignup, e error) {
			return nil, errors.New("update failed")
		}

		val, err := s.Application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
		require.NoError(s.T(), err)

		updated, err := s.Application.SignupService().UpdateUserSignup(val)
		require.EqualError(s.T(), err, "update failed")
		require.Nil(s.T(), updated)
	})
}

func (s *TestSignupServiceSuite) TestIsPhoneVerificationRequired() {
	test2.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, configuration.Namespace())

	s.Run("phone verification is required", func() {
		s.Run("captcha verification is disabled", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(false))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
		})

		s.Run("nil request", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
		})

		s.Run("request missing Recaptcha-Token header", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{Request: &http.Request{}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
		})

		s.Run("request Recaptcha-Token header incorrect length", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123", "456"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
		})

		s.Run("captcha assessment error", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{result: fmt.Errorf("assessment failed")}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
		})

		s.Run("captcha is enabled but the score is too low", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().CaptchaScoreThreshold("0.8"))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{score: 0.5}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(0.5), score, 0.01)
		})
	})

	s.Run("phone verification is not required", func() {
		s.Run("overall verification is disabled", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(false))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, nil)
			assert.False(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
		})
		s.Run("user's email domain is excluded", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().ExcludedEmailDomains("redhat.com"))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{Keys: map[string]interface{}{"email": "joe@redhat.com"}})
			assert.False(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
		})
		s.Run("captcha is enabled and the assessment is successful", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().CaptchaScoreThreshold("0.8"))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{score: 1.0}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.False(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(1.0), score, 0.01)
		})

	})

}

func (s *TestSignupServiceSuite) TestGetSignupUpdatesUserSignupIdentityClaims() {

	s.ServiceConfiguration(configuration.Namespace(), false, "", 5)

	// Create a new UserSignup, set its UserID and AccountID annotations
	userSignup := s.newUserSignupComplete()

	err := s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	mur := &toolchainv1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userSignup.Status.CompliantUsername,
			Namespace: configuration.Namespace(),
		},
		Spec: toolchainv1alpha1.MasterUserRecordSpec{
			UserAccounts: []toolchainv1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
		},
		Status: toolchainv1alpha1.MasterUserRecordStatus{
			Conditions: []toolchainv1alpha1.Condition{
				{
					Type:   toolchainv1alpha1.MasterUserRecordReady,
					Status: "true",
				},
			},
		},
	}
	err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
	require.NoError(s.T(), err)

	s.Run("PreferredUsername property updated when set in context", func() {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(context.UsernameKey, "cocochanel")

		_, err := s.Application.SignupService().GetSignup(c, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername)
		require.NoError(s.T(), err)

		modified, err := s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "cocochanel", modified.Spec.IdentityClaims.PreferredUsername)
		require.Equal(s.T(), "John", modified.Spec.IdentityClaims.GivenName)
		require.Equal(s.T(), "Smith", modified.Spec.IdentityClaims.FamilyName)
		require.Equal(s.T(), "Acme Inc", modified.Spec.IdentityClaims.Company)

		require.Equal(s.T(), "65432111", modified.Spec.IdentityClaims.AccountID)
		require.Equal(s.T(), "123456789", modified.Spec.IdentityClaims.Sub)
		require.Equal(s.T(), "54321666", modified.Spec.IdentityClaims.UserID)
		require.Equal(s.T(), "jsmith-original-sub", modified.Spec.IdentityClaims.OriginalSub)
		require.Equal(s.T(), "jsmith@redhat.com", modified.Spec.IdentityClaims.Email)
		require.Equal(s.T(), "90cb861692508c36933b85dfe43f5369", modified.Labels["toolchain.dev.openshift.com/email-hash"])

		s.Run("GivenName property updated when set in context", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set(context.GivenNameKey, "Jonathan")

			_, err := s.Application.SignupService().GetSignup(c, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername)
			require.NoError(s.T(), err)

			modified, err := s.FakeUserSignupClient.Get(userSignup.Name)
			require.NoError(s.T(), err)

			require.Equal(s.T(), "Jonathan", modified.Spec.IdentityClaims.GivenName)

			// Confirm that some other properties were not changed
			require.Equal(s.T(), "cocochanel", modified.Spec.IdentityClaims.PreferredUsername)
			require.Equal(s.T(), "Smith", modified.Spec.IdentityClaims.FamilyName)
			require.Equal(s.T(), "Acme Inc", modified.Spec.IdentityClaims.Company)

			require.Equal(s.T(), "65432111", modified.Spec.IdentityClaims.AccountID)
			require.Equal(s.T(), "123456789", modified.Spec.IdentityClaims.Sub)
			require.Equal(s.T(), "54321666", modified.Spec.IdentityClaims.UserID)
			require.Equal(s.T(), "jsmith-original-sub", modified.Spec.IdentityClaims.OriginalSub)
			require.Equal(s.T(), "jsmith@redhat.com", modified.Spec.IdentityClaims.Email)
			require.Equal(s.T(), "90cb861692508c36933b85dfe43f5369", modified.Labels["toolchain.dev.openshift.com/email-hash"])

			s.Run("FamilyName and Company properties updated when set in context", func() {
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Set(context.FamilyNameKey, "Smythe")
				c.Set(context.CompanyKey, "Red Hat")

				_, err := s.Application.SignupService().GetSignup(c, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername)
				require.NoError(s.T(), err)

				modified, err := s.FakeUserSignupClient.Get(userSignup.Name)
				require.NoError(s.T(), err)

				require.Equal(s.T(), "Smythe", modified.Spec.IdentityClaims.FamilyName)
				require.Equal(s.T(), "Red Hat", modified.Spec.IdentityClaims.Company)

				require.Equal(s.T(), "Jonathan", modified.Spec.IdentityClaims.GivenName)
				require.Equal(s.T(), "cocochanel", modified.Spec.IdentityClaims.PreferredUsername)

				require.Equal(s.T(), "65432111", modified.Spec.IdentityClaims.AccountID)
				require.Equal(s.T(), "123456789", modified.Spec.IdentityClaims.Sub)
				require.Equal(s.T(), "54321666", modified.Spec.IdentityClaims.UserID)
				require.Equal(s.T(), "jsmith-original-sub", modified.Spec.IdentityClaims.OriginalSub)
				require.Equal(s.T(), "jsmith@redhat.com", modified.Spec.IdentityClaims.Email)
				require.Equal(s.T(), "90cb861692508c36933b85dfe43f5369", modified.Labels["toolchain.dev.openshift.com/email-hash"])

				s.Run("Remaining properties updated when set in context", func() {
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Set(context.SubKey, "987654321")
					c.Set(context.UserIDKey, "123456777")
					c.Set(context.AccountIDKey, "777654321")
					c.Set(context.OriginalSubKey, "jsmythe-original-sub")
					c.Set(context.EmailKey, "jsmythe@redhat.com")

					_, err := s.Application.SignupService().GetSignup(c, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername)
					require.NoError(s.T(), err)

					modified, err := s.FakeUserSignupClient.Get(userSignup.Name)
					require.NoError(s.T(), err)

					require.Equal(s.T(), "987654321", modified.Spec.IdentityClaims.Sub)
					require.Equal(s.T(), "123456777", modified.Spec.IdentityClaims.UserID)
					require.Equal(s.T(), "777654321", modified.Spec.IdentityClaims.AccountID)
					require.Equal(s.T(), "jsmythe-original-sub", modified.Spec.IdentityClaims.OriginalSub)
					require.Equal(s.T(), "jsmythe@redhat.com", modified.Spec.IdentityClaims.Email)
					require.Equal(s.T(), "7cd294acda3a75773834df81d6e8ed7c", modified.Labels["toolchain.dev.openshift.com/email-hash"])

					require.Equal(s.T(), "Smythe", modified.Spec.IdentityClaims.FamilyName)
					require.Equal(s.T(), "Red Hat", modified.Spec.IdentityClaims.Company)
					require.Equal(s.T(), "Jonathan", modified.Spec.IdentityClaims.GivenName)
					require.Equal(s.T(), "cocochanel", modified.Spec.IdentityClaims.PreferredUsername)
				})
			})
		})
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
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailHashLabelKey: "90cb861692508c36933b85dfe43f5369",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PropagatedClaims: toolchainv1alpha1.PropagatedClaims{
					Sub:         "123456789",
					UserID:      "54321666",
					AccountID:   "65432111",
					OriginalSub: "jsmith-original-sub",
					Email:       "jsmith@redhat.com",
				},
				PreferredUsername: "jsmith",
				GivenName:         "John",
				FamilyName:        "Smith",
				Company:           "Acme Inc",
			},
		},
		Status: toolchainv1alpha1.UserSignupStatus{
			ScheduledDeactivationTimestamp: util.Ptr(v1.NewTime(time.Now().Add(30 * time.Hour * 24))),
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

func (s *TestSignupServiceSuite) newBannedUserSignup() *toolchainv1alpha1.UserSignup {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	return &toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailHashLabelKey: "a7b1b413c1cbddbcd19a51222ef8e20a",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PropagatedClaims: toolchainv1alpha1.PropagatedClaims{
					Sub:         "123456789",
					UserID:      "54321666",
					AccountID:   "65432111",
					OriginalSub: "jsmith-original-sub",
					Email:       "jsmith@gmail.com",
				},
				PreferredUsername: "jsmith",
				GivenName:         "John",
				FamilyName:        "Smith",
				Company:           "Acme Inc",
			},
		},
		Status: toolchainv1alpha1.UserSignupStatus{
			Conditions: []toolchainv1alpha1.Condition{
				{
					Type:   toolchainv1alpha1.UserSignupApproved,
					Status: apiv1.ConditionTrue,
					Reason: toolchainv1alpha1.UserSignupApprovedAutomaticallyReason,
				},
				{
					Type:   toolchainv1alpha1.UserSignupComplete,
					Status: apiv1.ConditionTrue,
					Reason: toolchainv1alpha1.UserSignupUserBannedReason,
				},
			},
			CompliantUsername: "jsmith",
		},
	}
}

func (s *TestSignupServiceSuite) newProvisionedMUR(name string) *toolchainv1alpha1.MasterUserRecord {
	return &toolchainv1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: configuration.Namespace(),
		},
		Spec: toolchainv1alpha1.MasterUserRecordSpec{
			UserAccounts: []toolchainv1alpha1.UserAccountEmbedded{{TargetCluster: "member-123"}},
		},
		Status: toolchainv1alpha1.MasterUserRecordStatus{
			ProvisionedTime: util.Ptr(v1.NewTime(time.Now())),
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

func (s *TestSignupServiceSuite) newSpaceForMUR(murName, creator string) *toolchainv1alpha1.Space {
	return &toolchainv1alpha1.Space{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      murName,
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.SpaceCreatorLabelKey: creator,
			},
		},
		Spec: toolchainv1alpha1.SpaceSpec{
			TargetCluster:      "member-123",
			TargetClusterRoles: []string{"cluster-role.toolchain.dev.openshift.com/tenant"},
			TierName:           "base1ns",
		},
		Status: toolchainv1alpha1.SpaceStatus{
			TargetCluster: "member-123",
			ProvisionedNamespaces: []toolchainv1alpha1.SpaceNamespace{
				{
					Name: fmt.Sprintf("%s-dev", murName),
					Type: "default",
				},
			},
		},
	}
}

func (s *TestSignupServiceSuite) newSpaceBinding(murName, spaceName string) *toolchainv1alpha1.SpaceBinding {
	name, err := uuid.NewV4()
	require.NoError(s.T(), err)

	return &toolchainv1alpha1.SpaceBinding{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      name.String(),
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.SpaceBindingSpaceLabelKey:            spaceName,
				toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey: murName,
			},
		},
		Spec: toolchainv1alpha1.SpaceBindingSpec{
			MasterUserRecord: murName,
			Space:            spaceName,
			SpaceRole:        "admin",
		},
	}
}

func deactivated() []toolchainv1alpha1.Condition {
	return []toolchainv1alpha1.Condition{
		{
			Type:   toolchainv1alpha1.UserSignupComplete,
			Status: apiv1.ConditionTrue,
			Reason: toolchainv1alpha1.UserSignupUserDeactivatedReason,
		},
		{
			Type:   toolchainv1alpha1.UserSignupApproved,
			Status: apiv1.ConditionFalse,
			Reason: toolchainv1alpha1.UserSignupUserDeactivatedReason,
		},
	}
}

type FakeCaptchaChecker struct {
	score  float32
	result error
}

func (c FakeCaptchaChecker) CompleteAssessment(_ *gin.Context, _ configuration.RegistrationServiceConfig, _ string) (float32, error) {
	return c.score, c.result
}
