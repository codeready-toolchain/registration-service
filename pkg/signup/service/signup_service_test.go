package service_test

import (
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	errors2 "github.com/codeready-toolchain/registration-service/pkg/errors"
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
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		require.Equal(s.T(), userID.String(), val.Spec.Userid)
		require.Equal(s.T(), username, val.Name)
		require.Equal(s.T(), username, val.Spec.Username)
		require.Equal(s.T(), "jane", val.Spec.GivenName)
		require.Equal(s.T(), "doe", val.Spec.FamilyName)
		require.Equal(s.T(), "red hat", val.Spec.Company)
		require.True(s.T(), states.VerificationRequired(&val))
		require.Equal(s.T(), "jsmith@gmail.com", val.Annotations[toolchainv1alpha1.UserSignupUserEmailAnnotationKey])
		require.Equal(s.T(), "13349822", val.Annotations[toolchainv1alpha1.SSOUserIDAnnotationKey])
		require.Equal(s.T(), "45983711", val.Annotations[toolchainv1alpha1.SSOAccountIDAnnotationKey])
		require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])
		require.Empty(s.T(), val.Annotations[toolchainv1alpha1.SkipAutoCreateSpaceAnnotationKey]) // skip auto create space annotation is not set by default

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
	require.Equal(s.T(), "original-sub-value", userSignup.Spec.OriginalSub)

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

	s.FakeUserSignupClient.MockGet = func(id string) (*toolchainv1alpha1.UserSignup, error) {
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

	// when
	_, err := s.Application.SignupService().GetSignup("000", "abc")

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
		_, err := svc.GetSignupFromInformer("000", "abc")

		// then
		require.EqualError(s.T(), err, "something quite unfortunate happened: something bad")
	})
}

func (s *TestSignupServiceSuite) TestSignupNoSpaces() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

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
	test2.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, TestNamespace)

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
	s.ServiceConfiguration(TestNamespace, true, "", 5)

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
	require.EqualError(s.T(), err, "forbidden: failed to create usersignup for jsmith-crtadmin")
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
	e := &apierrors.StatusError{}
	require.True(s.T(), errors.As(err, &e))
	require.Equal(s.T(), "Failure", e.ErrStatus.Status)
	require.Equal(s.T(), "forbidden: user has been banned", e.ErrStatus.Message)
	require.Equal(s.T(), v1.StatusReasonForbidden, e.ErrStatus.Reason)
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
	err = s.Application.SignupService().PhoneNumberAlreadyInUse(bannedUserID.String(), "jsmith", "+12268213044")
	require.EqualError(s.T(), err, "cannot re-register with phone number: phone number already in use")
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
	err = s.Application.SignupService().PhoneNumberAlreadyInUse(newUserID.String(), "jsmith", "+12268213044")
	require.EqualError(s.T(), err, "cannot re-register with phone number: phone number already in use")
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
	require.Equal(s.T(), "jsmith", val.Name)
	require.Equal(s.T(), "jsmith", val.Spec.Username)
	require.Equal(s.T(), userID.String(), val.Spec.Userid)
	require.Equal(s.T(), "", val.Spec.GivenName)
	require.Equal(s.T(), "", val.Spec.FamilyName)
	require.Equal(s.T(), "", val.Spec.Company)
	require.False(s.T(), states.ApprovedManually(&val))
	require.Equal(s.T(), "jsmith@gmail.com", val.Annotations[toolchainv1alpha1.UserSignupUserEmailAnnotationKey])
	require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])
}

func (s *TestSignupServiceSuite) TestGetUserSignupFails() {
	// given
	username := "johnsmith"

	s.FakeUserSignupClient.MockGet = func(name string) (*toolchainv1alpha1.UserSignup, error) {
		if name == username {
			return nil, errors.New("an error occurred")
		}
		return &toolchainv1alpha1.UserSignup{}, nil
	}

	// when
	_, err := s.Application.SignupService().GetSignup("", username)

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
		_, err := svc.GetSignupFromInformer("johnsmith", "abc")

		// then
		require.EqualError(s.T(), err, "an error occurred")
	})
}

func (s *TestSignupServiceSuite) TestGetSignupNotFound() {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	signup, err := s.Application.SignupService().GetSignup(userID.String(), "")
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
		signup, err := svc.GetSignupFromInformer(userID.String(), "")

		// then
		require.Nil(s.T(), signup)
		require.NoError(s.T(), err)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupStatusNotComplete() {
	// given
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

	// when
	response, err := s.Application.SignupService().GetSignup(userID.String(), "")

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), userID.String(), response.Name)
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
	require.Equal(s.T(), "", response.ProxyURL)

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
		response, err := svc.GetSignupFromInformer(userID.String(), "")

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), response)

		require.Equal(s.T(), userID.String(), response.Name)
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
		require.Equal(s.T(), "", response.ProxyURL)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupNoStatusNotCompleteCondition() {
	// given
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

		// when
		response, err := s.Application.SignupService().GetSignup(userID.String(), "")

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), response)

		require.Equal(s.T(), userID.String(), response.Name)
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
		require.Equal(s.T(), "", response.ProxyURL)

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
			response, err := svc.GetSignupFromInformer(userID.String(), "")

			// then
			require.NoError(s.T(), err)
			require.NotNil(s.T(), response)

			require.Equal(s.T(), userID.String(), response.Name)
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
			require.Equal(s.T(), "", response.ProxyURL)
		})
	}
}

func (s *TestSignupServiceSuite) TestGetSignupDeactivated() {
	// given
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupCompleteWithReason("Deactivated")
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	// when
	signup, err := s.Application.SignupService().GetSignup(us.Name, "")

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
		signup, err := svc.GetSignupFromInformer(us.Name, "")

		// then
		require.Nil(s.T(), signup)
		require.NoError(s.T(), err)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupStatusOK() {
	// given
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	mur := s.newProvisionedMUR()
	err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
	require.NoError(s.T(), err)

	toolchainStatus := &toolchainv1alpha1.ToolchainStatus{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "toolchain-status",
			Namespace: TestNamespace,
		},
		Status: toolchainv1alpha1.ToolchainStatusStatus{
			Members: []toolchainv1alpha1.Member{
				{
					ClusterName: "member-1",
					APIEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL:      "https://console.member-1.com",
							CheDashboardURL: "http://che-toolchain-che.member-1.com",
						},
					},
				},
				{
					ClusterName: "member-123",
					APIEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL:      "https://console.member-123.com",
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
	err = s.FakeToolchainStatusClient.Tracker.Add(toolchainStatus)
	require.NoError(s.T(), err)

	// when
	response, err := s.Application.SignupService().GetSignup(us.Name, "")

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), us.Name, response.Name)
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
	assert.Equal(s.T(), "https://proxy-url.com", response.ProxyURL)

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

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		response, err := svc.GetSignupFromInformer(us.Name, "")

		// then
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
		assert.Equal(s.T(), "https://proxy-url.com", response.ProxyURL)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupByUsernameOK() {
	// given
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	us.Name = service.EncodeUserIdentifier(us.Spec.Username)
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	mur := s.newProvisionedMUR()
	err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
	require.NoError(s.T(), err)

	toolchainStatus := &toolchainv1alpha1.ToolchainStatus{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "toolchain-status",
			Namespace: TestNamespace,
		},
		Status: toolchainv1alpha1.ToolchainStatusStatus{
			Members: []toolchainv1alpha1.Member{
				{
					ClusterName: "member-1",
					APIEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL:      "https://console.member-1.com",
							CheDashboardURL: "http://che-toolchain-che.member-1.com",
						},
					},
				},
				{
					ClusterName: "member-123",
					APIEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL:      "https://console.member-123.com",
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
	err = s.FakeToolchainStatusClient.Tracker.Add(toolchainStatus)
	require.NoError(s.T(), err)

	// when
	response, err := s.Application.SignupService().GetSignup("foo", us.Spec.Username)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), us.Name, response.Name)
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
	assert.Equal(s.T(), "https://proxy-url.com", response.ProxyURL)

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

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		response, err := svc.GetSignupFromInformer("foo", us.Spec.Username)

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), response)

		require.Equal(s.T(), us.Name, response.Name)
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
		assert.Equal(s.T(), "https://proxy-url.com", response.ProxyURL)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupStatusFailGetToolchainStatus() {
	// given
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	mur := s.newProvisionedMUR()
	err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
	require.NoError(s.T(), err)

	// when
	_, err = s.Application.SignupService().GetSignup(us.Name, "")

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
		_, err := svc.GetSignupFromInformer(us.Name, "")

		// then
		require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s:  \"toolchain-status\" not found", us.Name))
	})
}

func (s *TestSignupServiceSuite) TestGetSignupMURGetFails() {
	// given
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

	// when
	_, err = s.Application.SignupService().GetSignup(us.Name, "")

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
		_, err := svc.GetSignupFromInformer(us.Name, "")

		// then
		require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", us.Name))
	})
}

func (s *TestSignupServiceSuite) TestGetSignupUnknownStatus() {
	// given
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	mur := &toolchainv1alpha1.MasterUserRecord{
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
	}
	err = s.FakeMasterUserRecordClient.Tracker.Add(mur)
	require.NoError(s.T(), err)

	// when
	_, err = s.Application.SignupService().GetSignup(us.Name, "")

	// then
	require.EqualError(s.T(), err, "unable to parse readiness status as bool: blah-blah-blah: strconv.ParseBool: parsing \"blah-blah-blah\": invalid syntax")

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

		s.Application.MockInformerService(inf)
		svc := service.NewSignupService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
		)

		// when
		_, err := svc.GetSignupFromInformer(us.Name, "")

		// then
		require.EqualError(s.T(), err, "unable to parse readiness status as bool: blah-blah-blah: strconv.ParseBool: parsing \"blah-blah-blah\": invalid syntax")
	})
}

func (s *TestSignupServiceSuite) TestGetUserSignup() {
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	s.Run("getusersignup ok", func() {
		us := s.newUserSignupComplete()
		err := s.FakeUserSignupClient.Tracker.Add(us)
		require.NoError(s.T(), err)

		val, err := s.Application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
		require.NoError(s.T(), err)
		require.Equal(s.T(), us.Name, val.Name)
	})

	s.Run("getusersignup returns error", func() {
		s.FakeUserSignupClient.MockGet = func(s string) (userSignup *toolchainv1alpha1.UserSignup, e error) {
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
	s.ServiceConfiguration(TestNamespace, true, "", 5)

	us := s.newUserSignupComplete()
	err := s.FakeUserSignupClient.Tracker.Add(us)
	require.NoError(s.T(), err)

	s.Run("updateusersignup ok", func() {
		val, err := s.Application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
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

		val, err := s.Application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
		require.NoError(s.T(), err)

		updated, err := s.Application.SignupService().UpdateUserSignup(val)
		require.EqualError(s.T(), err, "update failed")
		require.Nil(s.T(), updated)
	})
}

func (s *TestSignupServiceSuite) TestIsPhoneVerificationRequired() {
	test2.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, TestNamespace)

	s.Run("phone verification is required", func() {
		s.Run("captcha verification is disabled", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(false))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{})
			assert.True(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(-1), score)
		})

		s.Run("nil request", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{})
			assert.True(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(-1), score)
		})

		s.Run("request missing Recaptcha-Token header", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{Request: &http.Request{}})
			assert.True(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(-1), score)
		})

		s.Run("request Recaptcha-Token header incorrect length", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123", "456"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(-1), score)
		})

		s.Run("captcha assessment error", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{result: fmt.Errorf("assessment failed")}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(-1), score)
		})

		s.Run("captcha is enabled but the score is too low", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().CaptchaScoreThreshold("0.8"))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{score: 0.5}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(0.5), score)
		})
	})

	s.Run("phone verification is not required", func() {
		s.Run("overall verification is disabled", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(false))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, nil)
			assert.False(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(-1), score)
		})
		s.Run("user's email domain is excluded", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().ExcludedEmailDomains("redhat.com"))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(nil, &gin.Context{Keys: map[string]interface{}{"email": "joe@redhat.com"}})
			assert.False(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(-1), score)
		})
		s.Run("captcha is enabled and the assessment is successful", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().CaptchaScoreThreshold("0.8"))

			isVerificationRequired, score := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{score: 1.0}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.False(s.T(), isVerificationRequired)
			assert.Equal(s.T(), float32(1.0), score)
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

func deactivated() []toolchainv1alpha1.Condition {
	return []toolchainv1alpha1.Condition{
		{
			Type:   toolchainv1alpha1.UserSignupComplete,
			Status: apiv1.ConditionTrue,
			Reason: "Deactivated",
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
