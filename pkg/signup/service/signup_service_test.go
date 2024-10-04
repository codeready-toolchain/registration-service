package service_test

import (
	"bytes"
	gocontext "context"
	"errors"
	"fmt"
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
	infservice "github.com/codeready-toolchain/registration-service/pkg/informers/service"
	"github.com/codeready-toolchain/registration-service/pkg/signup/service"
	"github.com/codeready-toolchain/registration-service/pkg/util"
	"github.com/codeready-toolchain/registration-service/test"
	testutil "github.com/codeready-toolchain/registration-service/test/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	recaptchapb "cloud.google.com/go/recaptchaenterprise/v2/apiv1/recaptchaenterprisepb"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestSignupServiceSuite struct {
	test.UnitTestSuite
}

func TestRunSignupServiceSuite(t *testing.T) {
	suite.Run(t, &TestSignupServiceSuite{test.UnitTestSuite{}})
}

func (s *TestSignupServiceSuite) ServiceConfiguration(verificationEnabled bool,
	excludedDomains string, verificationCodeExpiresInMin int) {

	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().Enabled(verificationEnabled).
			Verification().CodeExpiresInMin(verificationCodeExpiresInMin).
			Verification().ExcludedEmailDomains(excludedDomains))
}

func (s *TestSignupServiceSuite) TestSignup() {
	s.ServiceConfiguration(true, "", 5)
	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	assertUserSignupExists := func(cl client.Client, username string) toolchainv1alpha1.UserSignup {

		userSignups := &toolchainv1alpha1.UserSignupList{}
		err := cl.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs))
		require.NoError(s.T(), err)
		require.Len(s.T(), userSignups.Items, 1)

		val := userSignups.Items[0]
		require.Equal(s.T(), commontest.HostOperatorNs, val.Namespace)
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

		return val
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

	fakeClient, application := testutil.PrepareInClusterApp(s.T())

	// when
	userSignup, err := application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // at this point, the activation counter annotation is not set
	assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey]) // at this point, the last target cluster annotation is not set
	require.Equal(s.T(), "original-sub-value", userSignup.Spec.IdentityClaims.OriginalSub)

	existing := assertUserSignupExists(fakeClient, "jsmith")

	s.Run("deactivate and reactivate again", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		deactivatedUS.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey] = "2"        // assume the user was activated 2 times already
		deactivatedUS.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey] = "member-3" // assume the user was targeted to member-3
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = deactivated()
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), deactivatedUS)

		// when
		deactivatedUS, err = application.SignupService().Signup(ctx)

		// then
		require.NoError(s.T(), err)
		assertUserSignupExists(fakeClient, "jsmith")
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
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), deactivatedUS)

		// when
		userSignup, err := application.SignupService().Signup(ctx)

		// then
		require.NoError(s.T(), err)
		assertUserSignupExists(fakeClient, "jsmith")
		assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // was initially missing, and was not set
		assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey]) // was initially missing, and was not set
	})

	s.Run("deactivate and try to reactivate but reactivation fails", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = deactivated()
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), deactivatedUS)
		fakeClient.MockUpdate = func(ctx gocontext.Context, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && obj.GetName() == "jsmith" {
				return errors.New("an error occurred")
			}
			return fakeClient.Client.Update(ctx, obj, opts...)
		}

		// when
		_, err = application.SignupService().Signup(ctx)

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

	fakeClient, application := testutil.PrepareInClusterApp(s.T())
	fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
		return errors2.NewInternalError(errors.New("an internal error"), "an internal error happened")
	}

	// when
	_, err = application.SignupService().Signup(ctx)
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

	fakeClient, application := testutil.PrepareInClusterApp(s.T())
	fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && key.Name != userID.String() {
			return errors2.NewInternalError(errors.New("something bad happened"), "something very bad happened")
		}
		return fakeClient.Client.Get(ctx, key, obj, opts...)
	}

	// when
	_, err = application.SignupService().Signup(ctx)
	require.EqualError(s.T(), err, "something bad happened: something very bad happened")
}

func (s *TestSignupServiceSuite) TestGetSignupFailsWithNotFoundThenOtherError() {
	// given
	fakeClient, application := testutil.PrepareInClusterApp(s.T())
	fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && key.Name != "000" {
			return errors2.NewInternalError(errors.New("something quite unfortunate happened"), "something bad")
		}
		return fakeClient.Client.Get(ctx, key, obj, opts...)
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// when
	_, err := application.SignupService().GetSignup(c, "000", "abc")

	// then
	require.EqualError(s.T(), err, "something quite unfortunate happened: something bad")

	s.T().Run("informer", func(t *testing.T) {
		// given
		fakeClient := commontest.NewFakeClient(t)
		fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && key.Name != "000" {
				return errors2.NewInternalError(errors.New("something quite unfortunate happened"), "something bad")
			}
			return fakeClient.Client.Get(ctx, key, obj, opts...)
		}
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

		// when
		_, err = svc.GetSignupFromInformer(c, "000", "abc", true)

		// then
		require.EqualError(t, err, "something quite unfortunate happened: something bad")
	})
}

func (s *TestSignupServiceSuite) TestSignupNoSpaces() {
	s.ServiceConfiguration(true, "", 5)

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

	fakeClient, application := testutil.PrepareInClusterApp(s.T())

	// when
	userSignup, err := application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	userSignups := &toolchainv1alpha1.UserSignupList{}
	err = fakeClient.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs))
	require.NoError(s.T(), err)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.Equal(s.T(), "true", val.Annotations[toolchainv1alpha1.SkipAutoCreateSpaceAnnotationKey]) // skip auto create space annotation is set
}

func (s *TestSignupServiceSuite) TestSignupWithCaptchaEnabled() {
	commontest.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, commontest.HostOperatorNs)

	// captcha is enabled
	serviceOption := func(svc *service.ServiceImpl) {
		svc.CaptchaChecker = FakeCaptchaChecker{score: 0.9} // score is above threshold
	}

	opt := func(serviceFactory *factory.ServiceFactory) {
		serviceFactory.WithSignupServiceOption(serviceOption)
	}

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

	fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), opt)

	// when
	userSignup, err := application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	userSignups := &toolchainv1alpha1.UserSignupList{}
	err = fakeClient.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs))
	require.NoError(s.T(), err)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.Equal(s.T(), "0.9", val.Annotations[toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey]) // captcha score annotation is set
}

func (s *TestSignupServiceSuite) TestUserSignupWithInvalidSubjectPrefix() {
	s.ServiceConfiguration(true, "", 5)

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

	fakeClient, application := testutil.PrepareInClusterApp(s.T())

	// when
	userSignup, err := application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	userSignups := &toolchainv1alpha1.UserSignupList{}
	err = fakeClient.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs))
	require.NoError(s.T(), err)
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
	s.ServiceConfiguration(true, "redhat.com", 5)

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

	fakeClient, application := testutil.PrepareInClusterApp(s.T())

	// when
	userSignup, err := application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	userSignups := &toolchainv1alpha1.UserSignupList{}
	err = fakeClient.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs))
	require.NoError(s.T(), err)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.False(s.T(), states.VerificationRequired(&val))
}

func (s *TestSignupServiceSuite) TestCRTAdminUserSignup() {
	s.ServiceConfiguration(true, "redhat.com", 5)

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
	_, application := testutil.PrepareInClusterApp(s.T())

	// when
	userSignup, err := application.SignupService().Signup(ctx)

	// then
	require.EqualError(s.T(), err, "forbidden: failed to create usersignup for jsmith-crtadmin")
	require.Nil(s.T(), userSignup)
}

func (s *TestSignupServiceSuite) TestFailsIfUserSignupNameAlreadyExists() {
	s.ServiceConfiguration(true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	signup := &toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: commontest.HostOperatorNs,
		},
		Spec: toolchainv1alpha1.UserSignupSpec{},
	}
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	_, application := testutil.PrepareInClusterApp(s.T(), signup)

	// when
	_, err = application.SignupService().Signup(ctx)

	// then
	require.EqualError(s.T(), err, fmt.Sprintf("Operation cannot be fulfilled on  \"\": UserSignup [id: %s; username: jsmith]. Unable to create UserSignup because there is already an active UserSignup with such ID", userID.String()))
}

func (s *TestSignupServiceSuite) TestFailsIfUserBanned() {
	s.ServiceConfiguration(true, "", 5)

	// given
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUser := &toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: commontest.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: "a7b1b413c1cbddbcd19a51222ef8e20a",
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "jsmith@gmail.com",
		},
	}
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	_, application := testutil.PrepareInClusterApp(s.T(), bannedUser)

	// when
	_, err = application.SignupService().Signup(ctx)

	// then
	require.Error(s.T(), err)
	e := &apierrors.StatusError{}
	require.ErrorAs(s.T(), err, &e)
	require.Equal(s.T(), "Failure", e.ErrStatus.Status)
	require.Equal(s.T(), "forbidden: user has been banned", e.ErrStatus.Message)
	require.Equal(s.T(), v1.StatusReasonForbidden, e.ErrStatus.Reason)
}

func (s *TestSignupServiceSuite) TestPhoneNumberAlreadyInUseBannedUser() {
	s.ServiceConfiguration(true, "redhat.com", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUser := &toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: commontest.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey:       "a7b1b413c1cbddbcd19a51222ef8e20a",
				toolchainv1alpha1.BannedUserPhoneNumberHashLabelKey: "fd276563a8232d16620da8ec85d0575f",
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "jane.doe@gmail.com",
		},
	}
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	_, application := testutil.PrepareInClusterApp(s.T(), bannedUser)

	// when
	err = application.SignupService().PhoneNumberAlreadyInUse(bannedUserID.String(), "jsmith", "+12268213044")

	// then
	require.EqualError(s.T(), err, "cannot re-register with phone number: phone number already in use")
}

func (s *TestSignupServiceSuite) TestPhoneNumberAlreadyInUseUserSignup() {
	s.ServiceConfiguration(true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	signup := &toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: commontest.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailHashLabelKey: "a7b1b413c1cbddbcd19a51222ef8e20a",
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "fd276563a8232d16620da8ec85d0575f",
				toolchainv1alpha1.UserSignupStateLabelKey:         toolchainv1alpha1.UserSignupStateLabelValueApproved,
			},
		},
	}
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	newUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	_, application := testutil.PrepareInClusterApp(s.T(), signup)

	// when
	err = application.SignupService().PhoneNumberAlreadyInUse(newUserID.String(), "jsmith", "+12268213044")

	// then
	require.EqualError(s.T(), err, "cannot re-register with phone number: phone number already in use")
}

func (s *TestSignupServiceSuite) TestOKIfOtherUserBanned() {
	s.ServiceConfiguration(true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUserID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	bannedUser := &toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      bannedUserID.String(),
			Namespace: commontest.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: "1df66fbb427ff7e64ac46af29cc74b71",
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "jane.doe@gmail.com",
		},
	}
	require.NoError(s.T(), err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, userID.String())
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	fakeClient, application := testutil.PrepareInClusterApp(s.T(), bannedUser)

	// when
	userSignup, err := application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	userSignups := &toolchainv1alpha1.UserSignupList{}
	err = fakeClient.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs))
	require.NoError(s.T(), err)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.Equal(s.T(), commontest.HostOperatorNs, val.Namespace)
	require.Equal(s.T(), "jsmith", val.Name)
	require.False(s.T(), states.ApprovedManually(&val))
	require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])
}

func (s *TestSignupServiceSuite) TestGetUserSignupFails() {
	// given
	username := "johnsmith"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	fakeClient, application := testutil.PrepareInClusterApp(s.T())
	fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && key.Name != username {
			return errors.New("an error occurred")
		}
		return fakeClient.Client.Get(ctx, key, obj, opts...)
	}

	// when
	_, err := application.SignupService().GetSignup(c, "", username)

	// then
	require.EqualError(s.T(), err, "an error occurred")

	s.T().Run("informer", func(t *testing.T) {
		// given
		fakeClient := commontest.NewFakeClient(t)
		fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if key.Name != username {
				return errors.New("an error occurred")
			}
			return fakeClient.Client.Get(ctx, key, obj, opts...)
		}

		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

		// when
		_, err = svc.GetSignupFromInformer(c, "johnsmith", "abc", true)

		// then
		require.EqualError(t, err, "an error occurred")
	})
}

func (s *TestSignupServiceSuite) TestGetSignupNotFound() {
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, application := testutil.PrepareInClusterApp(s.T())

	// when
	signup, err := application.SignupService().GetSignup(c, userID.String(), "")

	// then
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)

	s.T().Run("informer", func(t *testing.T) {
		// given
		fakeClient := commontest.NewFakeClient(t)
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

		// when
		signup, err := svc.GetSignupFromInformer(c, userID.String(), "", true)

		// then
		require.Nil(t, signup)
		require.NoError(t, err)
	})
}

func (s *TestSignupServiceSuite) TestGetSignupStatusNotComplete() {
	// given
	s.ServiceConfiguration(true, "", 5)

	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	userSignupNotComplete := &toolchainv1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userID.String(),
			Namespace: commontest.HostOperatorNs,
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
	states.SetVerificationRequired(userSignupNotComplete, true)

	_, application := testutil.PrepareInClusterApp(s.T(), userSignupNotComplete)

	// when
	response, err := application.SignupService().GetSignup(c, userID.String(), "")

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
	assert.Empty(s.T(), response.StartDate)
	assert.Empty(s.T(), response.EndDate)

	s.T().Run("informer - with check for usersignup complete condition", func(t *testing.T) {
		// given
		fakeClient := commontest.NewFakeClient(t, userSignupNotComplete)
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

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
		states.SetVerificationRequired(userSignupNotComplete, false)
		mur := s.newProvisionedMUR("bill")
		space := s.newSpace(mur.Name)
		spacebinding := s.newSpaceBinding(mur.Name, space.Name)
		toolchainStatus := s.newToolchainStatus(".apps.")

		fakeClient := commontest.NewFakeClient(t, userSignupNotComplete, mur, space, spacebinding, toolchainStatus)
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

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
	s.ServiceConfiguration(true, "", 5)

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

		userSignup := &toolchainv1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      userID.String(),
				Namespace: commontest.HostOperatorNs,
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "bill",
				},
			},
			Status: status,
		}

		states.SetVerificationRequired(userSignup, true)

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		// when
		response, err := application.SignupService().GetSignup(c, userID.String(), "bill")

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
			fakeClient := commontest.NewFakeClient(t, userSignup)
			svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

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
	s.ServiceConfiguration(true, "", 5)

	us := s.newUserSignupComplete()
	us.Status.Conditions = deactivated()

	fakeClient, application := testutil.PrepareInClusterApp(s.T(), us)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// when
	signup, err := application.SignupService().GetSignup(c, us.Name, "")

	// then
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)

	s.T().Run("informer", func(t *testing.T) {
		// given
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

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
			s.ServiceConfiguration(true, "", 5)

			us := s.newUserSignupComplete()
			mur := s.newProvisionedMUR("ted")
			toolchainStatus := s.newToolchainStatus(appsSubDomain)
			space := s.newSpace(mur.Name)
			spacebinding := s.newSpaceBinding(mur.Name, space.Name)

			_, application := testutil.PrepareInClusterApp(s.T(), us, mur, toolchainStatus, space, spacebinding)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			// when
			response, err := application.SignupService().GetSignup(c, us.Name, "")

			// then
			require.NoError(t, err)
			require.NotNil(t, response)

			require.Equal(t, us.Name, response.Name)
			require.Equal(t, "jsmith", response.Username)
			require.Equal(t, "ted", response.CompliantUsername)

			require.Equal(t, mur.Status.ProvisionedTime.UTC().Format(time.RFC3339), response.StartDate)
			require.Equal(t, us.Status.ScheduledDeactivationTimestamp.UTC().Format(time.RFC3339), response.EndDate)
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
				fakeClient := commontest.NewFakeClient(t, us, mur, toolchainStatus, space, spacebinding)
				svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

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
	s.ServiceConfiguration(true, "", 5)

	us := s.newUserSignupComplete()
	us.Name = service.EncodeUserIdentifier(us.Spec.IdentityClaims.PreferredUsername)
	// Set the scheduled deactivation timestamp 1 day in the future
	deactivationTimestamp := time.Now().Add(time.Hour * 24).Round(time.Second).UTC()
	us.Status.ScheduledDeactivationTimestamp = util.Ptr(v1.NewTime(deactivationTimestamp))

	mur := s.newProvisionedMUR("ted")
	// Set the provisioned time 29 days in the past
	provisionedTime := time.Now().Add(-time.Hour * 24 * 29).Round(time.Second)
	mur.Status.ProvisionedTime = util.Ptr(v1.NewTime(provisionedTime))

	space := s.newSpace(mur.Name)
	spacebinding := s.newSpaceBinding(mur.Name, space.Name)
	toolchainStatus := s.newToolchainStatus(".apps.")

	fakeClient := commontest.NewFakeClient(s.T(), us, mur, space, spacebinding, toolchainStatus)
	svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(s.T(), fakeClient))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// when
	response, err := svc.GetSignup(c, "foo", us.Spec.IdentityClaims.PreferredUsername)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	// Confirm the StartDate is the same as the provisionedTime
	require.Equal(s.T(), provisionedTime.UTC().Format(time.RFC3339), response.StartDate)

	// Confirm the end date is about the same as the deactivationTimestamp
	responseEndDate, err := time.ParseInLocation(time.RFC3339, response.EndDate, nil)
	require.NoError(s.T(), err)
	require.Equal(s.T(), deactivationTimestamp, responseEndDate)

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
		fakeClient := commontest.NewFakeClient(t, us, mur, toolchainStatus, space, spacebinding)
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

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
			Namespace: commontest.HostOperatorNs,
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
	s.ServiceConfiguration(true, "", 5)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	us := s.newUserSignupComplete()
	mur := s.newProvisionedMUR("ted")
	space := s.newSpace("ted")

	_, application := testutil.PrepareInClusterApp(s.T(), us, mur, space)

	// when
	_, err := application.SignupService().GetSignup(c, us.Name, "")

	// then
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s: toolchainstatuses.toolchain.dev.openshift.com \"toolchain-status\" not found", us.Name))

	s.T().Run("informer", func(t *testing.T) {
		// given
		fakeClient := commontest.NewFakeClient(t, us, mur, space)
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

		// when
		_, err = svc.GetSignupFromInformer(c, us.Name, "", true)

		// then
		require.EqualError(t, err, fmt.Sprintf("error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s: toolchainstatuses.toolchain.dev.openshift.com \"toolchain-status\" not found", us.Name))
	})
}

func (s *TestSignupServiceSuite) TestGetSignupMURGetFails() {
	// given
	s.ServiceConfiguration(true, "", 5)

	us := s.newUserSignupComplete()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	returnedErr := errors.New("an error occurred")
	fakeClient, application := testutil.PrepareInClusterApp(s.T(), us)
	fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*toolchainv1alpha1.MasterUserRecord); ok && key.Name == us.Status.CompliantUsername {
			return returnedErr
		}
		return fakeClient.Client.Get(ctx, key, obj, opts...)
	}

	// when
	_, err := application.SignupService().GetSignup(c, us.Name, "")

	// then
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", us.Name))

	s.T().Run("informer", func(t *testing.T) {
		// given
		fakeClient := commontest.NewFakeClient(t, us)
		fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*toolchainv1alpha1.MasterUserRecord); ok && key.Name == us.Status.CompliantUsername {
				return returnedErr
			}
			return fakeClient.Client.Get(ctx, key, obj, opts...)
		}
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

		// when
		_, err = svc.GetSignupFromInformer(c, us.Name, "", true)

		// then
		require.EqualError(t, err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", us.Name))
	})
}

func (s *TestSignupServiceSuite) TestGetSignupReadyConditionStatus() {
	// given
	s.ServiceConfiguration(true, "", 5)

	us := s.newUserSignupComplete()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	mur := &toolchainv1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ted",
			Namespace: commontest.HostOperatorNs,
		},
	}

	space := s.newSpace("ted")
	toolchainStatus := s.newToolchainStatus(".apps.")

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
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), us, mur, space, toolchainStatus)

			// when
			response, err := application.SignupService().GetSignup(c, us.Name, "")

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedConditionReady, response.Status.Ready)
			require.Equal(t, tc.condition.Reason, response.Status.Reason)
			require.Equal(t, tc.condition.Message, response.Status.Message)

			// informer case
			// given
			svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

			// when
			_, err = svc.GetSignupFromInformer(c, us.Name, "", true)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedConditionReady, response.Status.Ready)
			require.Equal(t, tc.condition.Reason, response.Status.Reason)
			require.Equal(t, tc.condition.Message, response.Status.Message)
		})
	}
}

func (s *TestSignupServiceSuite) TestGetSignupBannedUserEmail() {
	// given
	s.ServiceConfiguration(true, "", 5)

	us := s.newBannedUserSignup()
	fakeClient, application := testutil.PrepareInClusterApp(s.T(), us)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, us.Spec.IdentityClaims.UserID)
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	// when
	response, err := application.SignupService().GetSignup(ctx, us.Name, "")

	// then
	// return not found signup
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)
	require.Equal(s.T(), toolchainv1alpha1.UserSignupPendingApprovalReason, response.Status.Reason)

	s.T().Run("informer", func(t *testing.T) {
		// given
		svc := service.NewSignupService(testutil.NewMemberClusterServiceContext(t, fakeClient))

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
	s.ServiceConfiguration(true, "", 5)

	space := s.newSpace("dave")
	fakeClient := commontest.NewFakeClient(s.T(), space)
	inf := infservice.NewInformerService(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(inf, "dave", "dave")

	// then
	assert.Equal(s.T(), "dave-dev", defaultUserNamespace)
	assert.Equal(s.T(), "member-123", targetCluster)
}

// TestGetDefaultUserNamespaceFromFirstUnownedSpace tests that the default user namespace is returned even if the only accessible Space was not created as the home space.
// This is valuable when user doesn't have default home space created, but has access to some shared spaces
func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceFromFirstUnownedSpace() {
	// given
	s.ServiceConfiguration(true, "", 5)
	// space created for userA
	space := s.newSpace("userA")
	// space shared with userB
	spacebindingB := s.newSpaceBinding("userB", space.Name)
	// space created for userC
	spaceC := s.newSpace("userC")
	// spaceC shared with userB
	spaceCindingC := s.newSpaceBinding("userB", spaceC.Name)

	fakeClient := commontest.NewFakeClient(s.T(), space, spacebindingB, spaceC, spaceCindingC)
	inf := infservice.NewInformerService(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(inf, "", "userB")

	// then
	assert.Equal(s.T(), "userA-dev", defaultUserNamespace)
	assert.Equal(s.T(), "member-123", targetCluster)
}

// TestGetDefaultUserNamespaceMultiSpace tests that the home Space created for the user is prioritized when there are multiple spaces
func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceMultiSpace() {
	// given
	s.ServiceConfiguration(true, "", 5)

	// space1 created by userA
	space1 := s.newSpace("userA")
	// space1 shared with userB
	spacebinding1 := s.newSpaceBinding("userB", space1.Name)
	// space2 created by userB
	space2 := s.newSpace("userB")
	// space2 shared with userB
	spacebinding2 := s.newSpaceBinding("userB", space2.Name)

	fakeClient := commontest.NewFakeClient(s.T(), space1, space2, spacebinding1, spacebinding2)
	inf := infservice.NewInformerService(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(inf, "userB", "userB")

	// then
	assert.Equal(s.T(), "userB-dev", defaultUserNamespace) // space2 is prioritized over space1 because it was created by the userB
	assert.Equal(s.T(), "member-123", targetCluster)

}

func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceFailNoHomeSpaceNoSpaceBinding() {
	// given
	s.ServiceConfiguration(true, "", 5)

	space := s.newSpace("dave")
	fakeClient := commontest.NewFakeClient(s.T(), space)
	inf := infservice.NewInformerService(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(inf, "", "dave")

	// then
	assert.Empty(s.T(), defaultUserNamespace)
	assert.Empty(s.T(), targetCluster)
}

func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceFailNoSpace() {
	// given
	s.ServiceConfiguration(true, "", 5)
	fakeClient := commontest.NewFakeClient(s.T())
	inf := infservice.NewInformerService(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(inf, "dave", "dave")

	// then
	assert.Empty(s.T(), defaultUserNamespace)
	assert.Empty(s.T(), targetCluster)
}

func (s *TestSignupServiceSuite) TestGetUserSignup() {
	s.ServiceConfiguration(true, "", 5)

	s.Run("getusersignup ok", func() {
		us := s.newUserSignupComplete()
		_, application := testutil.PrepareInClusterApp(s.T(), us)

		val, err := application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
		require.NoError(s.T(), err)
		require.Equal(s.T(), us.Name, val.Name)
	})

	s.Run("getusersignup returns error", func() {
		fakeClient, application := testutil.PrepareInClusterApp(s.T())
		fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok {
				return errors.New("get failed")
			}
			return fakeClient.Client.Get(ctx, key, obj, opts...)
		}

		val, err := application.SignupService().GetUserSignupFromIdentifier("foo", "")
		require.EqualError(s.T(), err, "get failed")
		require.Nil(s.T(), val)
	})

	s.Run("getusersignup with unknown user", func() {
		_, application := testutil.PrepareInClusterApp(s.T())

		val, err := application.SignupService().GetUserSignupFromIdentifier("unknown", "")
		require.True(s.T(), apierrors.IsNotFound(err))
		require.Nil(s.T(), val)
	})
}

func (s *TestSignupServiceSuite) TestUpdateUserSignup() {
	s.ServiceConfiguration(true, "", 5)

	us := s.newUserSignupComplete()

	s.Run("updateusersignup ok", func() {
		_, application := testutil.PrepareInClusterApp(s.T(), us)

		val, err := application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
		require.NoError(s.T(), err)

		val.Spec.IdentityClaims.FamilyName = "Johnson"

		updated, err := application.SignupService().UpdateUserSignup(val)
		require.NoError(s.T(), err)

		require.Equal(s.T(), val.Spec.IdentityClaims.FamilyName, updated.Spec.IdentityClaims.FamilyName)
	})

	s.Run("updateusersignup returns error", func() {
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), us)

		fakeClient.MockUpdate = func(ctx gocontext.Context, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok {
				return errors.New("update failed")
			}
			return fakeClient.Client.Update(ctx, obj, opts...)
		}

		val, err := application.SignupService().GetUserSignupFromIdentifier(us.Name, "")
		require.NoError(s.T(), err)

		updated, err := application.SignupService().UpdateUserSignup(val)
		require.EqualError(s.T(), err, "update failed")
		require.Nil(s.T(), updated)
	})
}

func (s *TestSignupServiceSuite) TestIsPhoneVerificationRequired() {
	commontest.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, commontest.HostOperatorNs)

	s.Run("phone verification is required", func() {
		s.Run("captcha verification is disabled", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(false))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, &gin.Context{})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Equal(s.T(), "", assessmentID)
		})

		s.Run("nil request", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, &gin.Context{})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Equal(s.T(), "", assessmentID)
		})

		s.Run("request missing Recaptcha-Token header", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, &gin.Context{Request: &http.Request{}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Equal(s.T(), "", assessmentID)
		})

		s.Run("request Recaptcha-Token header incorrect length", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123", "456"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Equal(s.T(), "", assessmentID)
		})

		s.Run("captcha assessment error", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{result: fmt.Errorf("assessment failed")}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Equal(s.T(), "", assessmentID)
		})

		s.Run("captcha is enabled but the score is too low", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().CaptchaScoreThreshold("0.8"))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{score: 0.5}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(0.5), score, 0.01)
			assert.Equal(s.T(), "captcha-assessment-123", assessmentID)
		})
	})

	s.Run("phone verification is not required", func() {
		s.Run("overall verification is disabled", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(false))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, nil)
			assert.False(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Equal(s.T(), "", assessmentID)
		})
		s.Run("user's email domain is excluded", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().ExcludedEmailDomains("redhat.com"))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, &gin.Context{Keys: map[string]interface{}{"email": "joe@redhat.com"}})
			assert.False(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Equal(s.T(), "", assessmentID)
		})
		s.Run("captcha is enabled and the assessment is successful", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true).
					Verification().CaptchaScoreThreshold("0.8"))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{score: 1.0}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.False(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(1.0), score, 0.01)
			assert.Equal(s.T(), "captcha-assessment-123", assessmentID)
		})

	})

}

func (s *TestSignupServiceSuite) TestGetSignupUpdatesUserSignupIdentityClaims() {

	s.ServiceConfiguration(false, "", 5)

	// Create a new UserSignup, set its UserID and AccountID annotations
	userSignup := s.newUserSignupComplete()

	mur := &toolchainv1alpha1.MasterUserRecord{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      userSignup.Status.CompliantUsername,
			Namespace: commontest.HostOperatorNs,
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

	s.Run("PreferredUsername property updated when set in context", func() {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(context.UsernameKey, "cocochanel")
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, mur)

		_, err := application.SignupService().GetSignup(c, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername)
		require.NoError(s.T(), err)

		modified := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), modified)
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

			_, err := application.SignupService().GetSignup(c, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername)
			require.NoError(s.T(), err)

			modified := &toolchainv1alpha1.UserSignup{}
			err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), modified)
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

				_, err := application.SignupService().GetSignup(c, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername)
				require.NoError(s.T(), err)

				modified := &toolchainv1alpha1.UserSignup{}
				err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), modified)
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

					_, err := application.SignupService().GetSignup(c, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername)
					require.NoError(s.T(), err)

					modified := &toolchainv1alpha1.UserSignup{}
					err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), modified)
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
			Namespace: commontest.HostOperatorNs,
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
			HomeSpace:         "ted",
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
			Namespace: commontest.HostOperatorNs,
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
			Namespace: commontest.HostOperatorNs,
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

func (s *TestSignupServiceSuite) newSpace(name string) *toolchainv1alpha1.Space {
	return &toolchainv1alpha1.Space{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: commontest.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.SpaceCreatorLabelKey: name,
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
					Name: fmt.Sprintf("%s-dev", name),
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
			Namespace: commontest.HostOperatorNs,
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

func (c FakeCaptchaChecker) CompleteAssessment(_ *gin.Context, _ configuration.RegistrationServiceConfig, _ string) (*recaptchapb.Assessment, error) {
	return &recaptchapb.Assessment{
		RiskAnalysis: &recaptchapb.RiskAnalysis{
			Score: c.score,
		},
		Name: "captcha-assessment-123",
	}, c.result
}
