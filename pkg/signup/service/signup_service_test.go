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

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	errors2 "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/registration-service/pkg/signup/service"
	"github.com/codeready-toolchain/registration-service/pkg/util"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"
	testutil "github.com/codeready-toolchain/registration-service/test/util"
	"github.com/codeready-toolchain/toolchain-common/pkg/test/masteruserrecord"
	testsocialevent "github.com/codeready-toolchain/toolchain-common/pkg/test/socialevent"
	"github.com/codeready-toolchain/toolchain-common/pkg/test/space"
	testusersignup "github.com/codeready-toolchain/toolchain-common/pkg/test/usersignup"
	signupcommon "github.com/codeready-toolchain/toolchain-common/pkg/usersignup"
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
	requestTime := time.Now()
	assertUserSignupExists := func(cl client.Client, username string) toolchainv1alpha1.UserSignup {

		userSignups := &toolchainv1alpha1.UserSignupList{}
		err := cl.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs))
		require.NoError(s.T(), err)
		require.Len(s.T(), userSignups.Items, 1)

		val := userSignups.Items[0]
		require.Equal(s.T(), commontest.HostOperatorNs, val.Namespace)
		require.Equal(s.T(), signupcommon.EncodeUserIdentifier(username), val.Name)
		require.True(s.T(), states.VerificationRequired(&val))
		require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])
		require.Empty(s.T(), val.Annotations[toolchainv1alpha1.SkipAutoCreateSpaceAnnotationKey]) // skip auto create space annotation is not set by default
		require.NotEmpty(s.T(), val.Annotations)
		require.Equal(s.T(), requestTime.Format(time.RFC3339), val.Annotations[toolchainv1alpha1.UserSignupRequestReceivedTimeAnnotationKey])

		// Confirm all the IdentityClaims have been correctly set
		require.Equal(s.T(), username, val.Spec.IdentityClaims.PreferredUsername)
		require.Equal(s.T(), "jane", val.Spec.IdentityClaims.GivenName)
		require.Equal(s.T(), "doe", val.Spec.IdentityClaims.FamilyName)
		require.Equal(s.T(), "red hat", val.Spec.IdentityClaims.Company)
		require.Equal(s.T(), "987654321", val.Spec.IdentityClaims.Sub)
		require.Equal(s.T(), "13349822", val.Spec.IdentityClaims.UserID)
		require.Equal(s.T(), "45983711", val.Spec.IdentityClaims.AccountID)
		require.Equal(s.T(), "123456789", val.Spec.IdentityClaims.AccountNumber)
		require.Equal(s.T(), "original-sub-value", val.Spec.IdentityClaims.OriginalSub)
		require.Equal(s.T(), "jsmith@gmail.com", val.Spec.IdentityClaims.Email)

		return val
	}

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith@kubesaw")
	ctx.Set(context.SubKey, "987654321")
	ctx.Set(context.OriginalSubKey, "original-sub-value")
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	ctx.Set(context.GivenNameKey, "jane")
	ctx.Set(context.FamilyNameKey, "doe")
	ctx.Set(context.CompanyKey, "red hat")
	ctx.Set(context.UserIDKey, "13349822")
	ctx.Set(context.AccountIDKey, "45983711")
	ctx.Set(context.AccountNumberKey, "123456789")
	ctx.Set(context.RequestReceivedTime, requestTime)

	fakeClient, application := testutil.PrepareInClusterApp(s.T())

	// when
	userSignup, err := application.SignupService().Signup(ctx)

	// then
	require.NoError(s.T(), err)
	assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // at this point, the activation counter annotation is not set
	assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey]) // at this point, the last target cluster annotation is not set
	require.Equal(s.T(), "original-sub-value", userSignup.Spec.IdentityClaims.OriginalSub)

	existing := assertUserSignupExists(fakeClient, "jsmith@kubesaw")

	s.Run("deactivate and reactivate again", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		deactivatedUS.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey] = "2"        // assume the user was activated 2 times already
		deactivatedUS.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey] = "member-3" // assume the user was targeted to member-3
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = fake.Deactivated()
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), deactivatedUS)

		// when
		deactivatedUS, err = application.SignupService().Signup(ctx)

		// then
		require.NoError(s.T(), err)
		assertUserSignupExists(fakeClient, "jsmith@kubesaw")
		assert.Equal(s.T(), "2", deactivatedUS.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey])        // value was preserved
		assert.Equal(s.T(), "member-3", deactivatedUS.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey]) // value was preserved
	})

	s.Run("deactivate and reactivate with missing annotation", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = fake.Deactivated()
		// also, alter the activation counter annotation
		delete(deactivatedUS.Annotations, toolchainv1alpha1.UserSignupActivationCounterAnnotationKey)
		delete(deactivatedUS.Annotations, toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey)
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), deactivatedUS)

		// when
		userSignup, err := application.SignupService().Signup(ctx)

		// then
		require.NoError(s.T(), err)
		assertUserSignupExists(fakeClient, "jsmith@kubesaw")
		assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]) // was initially missing, and was not set
		assert.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey]) // was initially missing, and was not set
	})

	s.Run("deactivate and try to reactivate but reactivation fails", func() {
		// given
		deactivatedUS := existing.DeepCopy()
		states.SetDeactivated(deactivatedUS, true)
		deactivatedUS.Status.Conditions = fake.Deactivated()
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), deactivatedUS)
		fakeClient.MockUpdate = func(ctx gocontext.Context, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && obj.GetName() == signupcommon.EncodeUserIdentifier("jsmith@kubesaw") {
				return errors.New("an error occurred")
			}
			return fakeClient.Client.Update(ctx, obj, opts...)
		}

		// when
		_, err = application.SignupService().Signup(ctx)

		// then
		require.EqualError(s.T(), err, "an error occurred")
	})

	s.Run("with social event code", func() {
		withCodeCtx := ctx.Copy()
		withCodeCtx.Set(context.SocialEvent, "event1")
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event1", testsocialevent.WithTargetCluster("event-member"))
		s.Run("set target cluster", func() {
			// when
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), event)

			// when
			returnedSignup, err := application.SignupService().Signup(withCodeCtx)

			// then
			require.NoError(s.T(), err)
			signup := &toolchainv1alpha1.UserSignup{}
			require.NoError(s.T(), fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(returnedSignup), signup))
			require.Equal(s.T(), "event-member", signup.Spec.TargetCluster)
			assert.True(s.T(), states.ApprovedManually(signup))
		})

		s.Run("set target cluster when reactivating", func() {
			// given
			deactivatedUS := existing.DeepCopy()
			states.SetDeactivated(deactivatedUS, true)
			deactivatedUS.Status.Conditions = fake.Deactivated()
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), deactivatedUS, event)

			// when
			reactivatedSignup, err := application.SignupService().Signup(withCodeCtx)

			// then
			require.NoError(s.T(), err)
			signup := &toolchainv1alpha1.UserSignup{}
			require.NoError(s.T(), fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(reactivatedSignup), signup))
			require.Equal(s.T(), "event-member", signup.Spec.TargetCluster)
			assert.False(s.T(), states.Deactivated(signup))
			assert.True(s.T(), states.ApprovedManually(signup))
		})

		s.Run("when event not present", func() {
			// when
			fakeClient, application := testutil.PrepareInClusterApp(s.T())

			// when
			returnedSignup, err := application.SignupService().Signup(withCodeCtx)

			// then
			require.Error(s.T(), err)
			require.Nil(s.T(), returnedSignup)
			userSignups := &toolchainv1alpha1.UserSignupList{}
			require.NoError(s.T(), fakeClient.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs)))
			require.Empty(s.T(), userSignups.Items)
		})
	})
}

func (s *TestSignupServiceSuite) TestSignupFailsWhenClientReturnsError() {

	// given
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "zoeabernathy")
	ctx.Set(context.SubKey, "987654321")
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
	_, err := application.SignupService().Signup(ctx)
	require.EqualError(s.T(), err, "an internal error: an internal error happened")
}

func (s *TestSignupServiceSuite) TestSignupFailsWithNotFoundThenOtherError() {

	// given
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "lisasmith")
	ctx.Set(context.SubKey, "987654321")
	ctx.Set(context.OriginalSubKey, "original-sub-value")
	ctx.Set(context.EmailKey, "lsmith@gmail.com")
	ctx.Set(context.GivenNameKey, "lisa")
	ctx.Set(context.FamilyNameKey, "smith")
	ctx.Set(context.CompanyKey, "red hat")

	fakeClient, application := testutil.PrepareInClusterApp(s.T())
	fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && key.Name == "lisasmith" {
			return errors2.NewInternalError(errors.New("something bad happened"), "something very bad happened")
		}
		return fakeClient.Client.Get(ctx, key, obj, opts...)
	}

	// when
	_, err := application.SignupService().Signup(ctx)
	require.EqualError(s.T(), err, "something bad happened: something very bad happened")
}

func (s *TestSignupServiceSuite) TestGetSignupFailsWithNotFoundThenOtherError() {
	// given
	fakeClient, application := testutil.PrepareInClusterApp(s.T())
	fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && key.Name == "abc" {
			return errors2.NewInternalError(errors.New("something quite unfortunate happened"), "something bad")
		}
		return fakeClient.Client.Get(ctx, key, obj, opts...)
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// when
	_, err := application.SignupService().GetSignup(c, "abc", true)

	// then
	require.EqualError(s.T(), err, "something quite unfortunate happened: something bad")
}

func (s *TestSignupServiceSuite) TestSignupNoSpaces() {
	s.ServiceConfiguration(true, "", 5)

	// given
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, "987654321")
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

	nsdClient := namespaced.NewClient(commontest.NewFakeClient(s.T()), commontest.HostOperatorNs)
	signupService := service.NewSignupService(nsdClient)
	// captcha is enabled
	signupService.CaptchaChecker = FakeCaptchaChecker{score: 0.9} // score is above threshold

	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().Enabled(true).
			Verification().CaptchaEnabled(true).
			Verification().CaptchaScoreThreshold("0.8"))

	// given
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, "987654321")
	ctx.Set(context.OriginalSubKey, "original-sub-value")
	ctx.Set(context.EmailKey, "jsmith@gmail.com")
	ctx.Set(context.GivenNameKey, "jane")
	ctx.Set(context.FamilyNameKey, "doe")
	ctx.Set(context.CompanyKey, "red hat")
	ctx.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString(""))
	ctx.Request.Header.Set("Recaptcha-Token", "abc")

	// when
	userSignup, err := signupService.Signup(ctx)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	userSignups := &toolchainv1alpha1.UserSignupList{}
	err = nsdClient.List(gocontext.TODO(), userSignups, client.InNamespace(commontest.HostOperatorNs))
	require.NoError(s.T(), err)
	require.Len(s.T(), userSignups.Items, 1)

	val := userSignups.Items[0]
	require.Equal(s.T(), "0.9", val.Annotations[toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey]) // captcha score annotation is set
}

func (s *TestSignupServiceSuite) TestUserSignupWithInvalidSubjectPrefix() {
	s.ServiceConfiguration(true, "", 5)

	// given
	username := "-sjones"

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, username)
	ctx.Set(context.SubKey, "987654321")
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

func (s *TestSignupServiceSuite) TestUserWithExcludedDomainEmailSignsUp() {
	s.ServiceConfiguration(true, "redhat.com", 5)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, "987654321")
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

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith-crtadmin")
	ctx.Set(context.SubKey, "987654321")
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

	signup := testusersignup.NewUserSignup(testusersignup.WithEncodedName("jsmith@kubesaw"))

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith@kubesaw")
	ctx.Set(context.SubKey, "userid")
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	_, application := testutil.PrepareInClusterApp(s.T(), signup)

	// when
	_, err := application.SignupService().Signup(ctx)

	// then
	require.EqualError(s.T(), err, "Operation cannot be fulfilled on  \"\": UserSignup [username: jsmith@kubesaw]. Unable to create UserSignup because there is already an active UserSignup with such a username")
}

func (s *TestSignupServiceSuite) TestFailsIfUserBanned() {
	s.ServiceConfiguration(true, "", 5)

	// given
	bannedUser := &toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "banned-user",
			Namespace: commontest.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: "a7b1b413c1cbddbcd19a51222ef8e20a",
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "jsmith@gmail.com",
		},
	}

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	_, application := testutil.PrepareInClusterApp(s.T(), bannedUser)

	// when
	response, err := application.SignupService().Signup(ctx)

	// then
	require.Error(s.T(), err)
	assert.Equal(s.T(), service.ForbiddenBannedError, err)
	require.Nil(s.T(), response)
}

func (s *TestSignupServiceSuite) TestOKIfOtherUserBanned() {
	s.ServiceConfiguration(true, "", 5)

	bannedUser := &toolchainv1alpha1.BannedUser{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "banneduser",
			Namespace: commontest.HostOperatorNs,
			Labels: map[string]string{
				toolchainv1alpha1.BannedUserEmailHashLabelKey: "1df66fbb427ff7e64ac46af29cc74b71",
			},
		},
		Spec: toolchainv1alpha1.BannedUserSpec{
			Email: "jane.doe@gmail.com",
		},
	}

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith@gmail")
	ctx.Set(context.SubKey, "userid")
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
	require.Equal(s.T(), signupcommon.EncodeUserIdentifier("jsmith@gmail"), val.Name)
	require.False(s.T(), states.ApprovedManually(&val))
	require.Equal(s.T(), "a7b1b413c1cbddbcd19a51222ef8e20a", val.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey])
}

func (s *TestSignupServiceSuite) TestGetUserSignupFails() {
	// given
	username := "johnsmith"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	fakeClient, application := testutil.PrepareInClusterApp(s.T())
	fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && key.Name == username {
			return errors.New("an error occurred")
		}
		return fakeClient.Client.Get(ctx, key, obj, opts...)
	}

	// when
	_, err := application.SignupService().GetSignup(c, username, true)

	// then
	require.EqualError(s.T(), err, "an error occurred")
}

func (s *TestSignupServiceSuite) TestGetSignupNotFound() {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, application := testutil.PrepareInClusterApp(s.T())

	// when
	signup, err := application.SignupService().GetSignup(c, "does-not-exist", true)

	// then
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusNotComplete() {
	// given
	s.ServiceConfiguration(true, "", 5)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	userSignupNotComplete := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("not-complete@kubesaw"),
		testusersignup.WithCompliantUsername("bill"),
		testusersignup.SignupIncomplete("test_reason", "test_message"),
		testusersignup.ApprovedAutomaticallyAgo(0),
	)
	states.SetVerificationRequired(userSignupNotComplete, true)

	_, application := testutil.PrepareInClusterApp(s.T(), userSignupNotComplete)

	// when
	response, err := application.SignupService().GetSignup(c, "not-complete@kubesaw", true)

	// then
	require.NoError(s.T(), err)
	require.NotNil(s.T(), response)

	require.Equal(s.T(), userSignupNotComplete.Name, response.Name)
	require.Equal(s.T(), "not-complete@kubesaw", response.Username)
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

	s.Run("with no check for UserSignup complete condition", func() {
		// given
		states.SetVerificationRequired(userSignupNotComplete, false)
		mur := s.newProvisionedMUR("bill")
		space := s.newSpace(mur.Name)
		spacebinding := s.newSpaceBinding(mur.Name, space.Name)
		toolchainStatus := s.newToolchainStatus(".apps.")

		fakeClient := commontest.NewFakeClient(s.T(), userSignupNotComplete, mur, space, spacebinding, toolchainStatus)
		svc := service.NewSignupService(namespaced.NewClient(fakeClient, commontest.HostOperatorNs))

		// when
		// we set checkUserSignupCompleted to false
		response, err := svc.GetSignup(c, "not-complete@kubesaw", false)

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), response)

		require.Equal(s.T(), userSignupNotComplete.Name, response.Name)
		require.Equal(s.T(), "not-complete@kubesaw", response.Username)
		require.Equal(s.T(), "bill", response.CompliantUsername)
		require.True(s.T(), response.Status.Ready)
		require.Equal(s.T(), "mur_ready_reason", response.Status.Reason)
		require.Equal(s.T(), "mur_ready_message", response.Status.Message)
		require.False(s.T(), response.Status.VerificationRequired)
		require.Equal(s.T(), "https://console.apps.member-123.com", response.ConsoleURL)
		require.Equal(s.T(), "https://devspaces.apps.member-123.com", response.CheDashboardURL)
		require.Equal(s.T(), "http://api.devcluster.openshift.com", response.APIEndpoint)
		require.Equal(s.T(), "member-123", response.ClusterName)
		require.Equal(s.T(), "https://proxy-url.com", response.ProxyURL)
		assert.Equal(s.T(), "bill-dev", response.DefaultUserNamespace)
		assert.Equal(s.T(), "https://rhods-dashboard-redhat-ods-applications.apps.member-123.com", response.RHODSMemberURL)
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
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		userSignup := &toolchainv1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "bill",
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
		response, err := application.SignupService().GetSignup(c, "bill", true)

		// then
		require.NoError(s.T(), err)
		require.NotNil(s.T(), response)

		require.Equal(s.T(), "bill", response.Name)
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
		assert.Empty(s.T(), response.DefaultUserNamespace)
		assert.Empty(s.T(), response.RHODSMemberURL)
	}
}

func (s *TestSignupServiceSuite) TestGetSignupDeactivated() {
	// given
	s.ServiceConfiguration(true, "", 5)

	username, us := s.newUserSignupComplete()
	us.Status.Conditions = fake.Deactivated()

	_, application := testutil.PrepareInClusterApp(s.T(), us)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// when
	signup, err := application.SignupService().GetSignup(c, username, true)

	// then
	require.Nil(s.T(), signup)
	require.NoError(s.T(), err)
}

func (s *TestSignupServiceSuite) TestGetSignupStatusOK() {
	// given
	for _, appsSubDomain := range []string{".apps.", ".apps-"} {
		s.SetupTest()
		s.Run("for apps subdomain: "+appsSubDomain, func() {
			s.ServiceConfiguration(true, "", 5)

			username, us := s.newUserSignupComplete()
			mur := s.newProvisionedMUR("ted")
			toolchainStatus := s.newToolchainStatus(appsSubDomain)
			space := s.newSpace(mur.Name)
			spacebinding := s.newSpaceBinding(mur.Name, space.Name)

			_, application := testutil.PrepareInClusterApp(s.T(), us, mur, toolchainStatus, space, spacebinding)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			// when
			response, err := application.SignupService().GetSignup(c, username, true)

			// then
			require.NoError(s.T(), err)
			require.NotNil(s.T(), response)

			require.Equal(s.T(), us.Name, response.Name)
			require.Equal(s.T(), username, response.Username)
			require.Equal(s.T(), us.Spec.IdentityClaims.GivenName, response.GivenName)
			require.Equal(s.T(), us.Spec.IdentityClaims.FamilyName, response.FamilyName)
			require.Equal(s.T(), us.Spec.IdentityClaims.Email, response.Email)
			require.Equal(s.T(), "ted", response.CompliantUsername)

			require.Equal(s.T(), mur.Status.ProvisionedTime.UTC().Format(time.RFC3339), response.StartDate)
			require.Equal(s.T(), us.Status.ScheduledDeactivationTimestamp.UTC().Format(time.RFC3339), response.EndDate)
			assert.True(s.T(), response.Status.Ready)
			assert.Equal(s.T(), "mur_ready_reason", response.Status.Reason)
			assert.Equal(s.T(), "mur_ready_message", response.Status.Message)
			assert.False(s.T(), response.Status.VerificationRequired)
			assert.Equal(s.T(), fmt.Sprintf("https://console%smember-123.com", appsSubDomain), response.ConsoleURL)
			assert.Equal(s.T(), fmt.Sprintf("https://devspaces%smember-123.com", appsSubDomain), response.CheDashboardURL)
			assert.Equal(s.T(), "http://api.devcluster.openshift.com", response.APIEndpoint)
			assert.Equal(s.T(), "member-123", response.ClusterName)
			assert.Equal(s.T(), "https://proxy-url.com", response.ProxyURL)
			assert.Equal(s.T(), "ted-dev", response.DefaultUserNamespace)
			assert.Equal(s.T(), fmt.Sprintf("https://rhods-dashboard-redhat-ods-applications%smember-123.com", appsSubDomain), response.RHODSMemberURL)
		})
	}
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
							ConsoleURL: fmt.Sprintf("https://console%smember-1.com", appsSubDomain),
						},
					},
				},
				{
					ClusterName: "member-123",
					APIEndpoint: "http://api.devcluster.openshift.com",
					MemberStatus: toolchainv1alpha1.MemberStatusStatus{
						Routes: &toolchainv1alpha1.Routes{
							ConsoleURL: fmt.Sprintf("https://console%smember-123.com", appsSubDomain),
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

	username, us := s.newUserSignupComplete()
	mur := s.newProvisionedMUR("ted")
	space := s.newSpace("ted")

	_, application := testutil.PrepareInClusterApp(s.T(), us, mur, space)

	// when
	_, err := application.SignupService().GetSignup(c, username, true)

	// then
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving ToolchainStatus for completed UserSignup %s: toolchainstatuses.toolchain.dev.openshift.com \"toolchain-status\" not found", us.Name))
}

func (s *TestSignupServiceSuite) TestGetSignupMURGetFails() {
	// given
	s.ServiceConfiguration(true, "", 5)

	username, us := s.newUserSignupComplete()

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
	_, err := application.SignupService().GetSignup(c, username, true)

	// then
	require.EqualError(s.T(), err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s: an error occurred", us.Name))
}

func (s *TestSignupServiceSuite) TestGetSignupReadyConditionStatus() {
	// given
	s.ServiceConfiguration(true, "", 5)

	username, us := s.newUserSignupComplete()

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
		s.Run(tcName, func() {

			// given
			mur.Status = toolchainv1alpha1.MasterUserRecordStatus{
				Conditions: []toolchainv1alpha1.Condition{
					tc.condition,
				},
			}
			_, application := testutil.PrepareInClusterApp(s.T(), us, mur, space, toolchainStatus)

			// when
			response, err := application.SignupService().GetSignup(c, username, true)

			// then
			require.NoError(s.T(), err)
			require.Equal(s.T(), tc.expectedConditionReady, response.Status.Ready)
			require.Equal(s.T(), tc.condition.Reason, response.Status.Reason)
			require.Equal(s.T(), tc.condition.Message, response.Status.Message)
		})
	}
}

func (s *TestSignupServiceSuite) TestGetSignupBannedUserEmail() {
	// given
	s.ServiceConfiguration(true, "", 5)

	us := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("ted@kubesaw"),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupUserEmailHashLabelKey, "a7b1b413c1cbddbcd19a51222ef8e20a"),
		testusersignup.ApprovedAutomaticallyAgo(time.Second),
		testusersignup.BannedAgo(time.Second),
		testusersignup.WithCompliantUsername("ted"))
	_, application := testutil.PrepareInClusterApp(s.T(), us)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Set(context.UsernameKey, "jsmith")
	ctx.Set(context.SubKey, us.Spec.IdentityClaims.UserID)
	ctx.Set(context.EmailKey, "jsmith@gmail.com")

	// when
	response, err := application.SignupService().GetSignup(ctx, "ted@kubesaw", true)

	// then
	require.Error(s.T(), err)
	assert.Equal(s.T(), service.ForbiddenBannedError, err)
	require.Nil(s.T(), response)
}

func (s *TestSignupServiceSuite) TestGetDefaultUserNamespace() {
	// given
	s.ServiceConfiguration(true, "", 5)

	space := s.newSpace("dave")
	fakeClient := commontest.NewFakeClient(s.T(), space)
	nsClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(nsClient, "dave", "dave")

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
	nsClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(nsClient, "", "userB")

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
	nsClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(nsClient, "userB", "userB")

	// then
	assert.Equal(s.T(), "userB-dev", defaultUserNamespace) // space2 is prioritized over space1 because it was created by the userB
	assert.Equal(s.T(), "member-123", targetCluster)

}

func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceFailNoHomeSpaceNoSpaceBinding() {
	// given
	s.ServiceConfiguration(true, "", 5)

	space := s.newSpace("dave")
	fakeClient := commontest.NewFakeClient(s.T(), space)
	nsClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(nsClient, "", "dave")

	// then
	assert.Empty(s.T(), defaultUserNamespace)
	assert.Empty(s.T(), targetCluster)
}

func (s *TestSignupServiceSuite) TestGetDefaultUserNamespaceFailNoSpace() {
	// given
	s.ServiceConfiguration(true, "", 5)
	fakeClient := commontest.NewFakeClient(s.T())
	nsClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

	// when
	targetCluster, defaultUserNamespace := service.GetDefaultUserTarget(nsClient, "dave", "dave")

	// then
	assert.Empty(s.T(), defaultUserNamespace)
	assert.Empty(s.T(), targetCluster)
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
			assert.Empty(s.T(), assessmentID)
		})

		s.Run("nil request", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, &gin.Context{})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Empty(s.T(), assessmentID)
		})

		s.Run("request missing Recaptcha-Token header", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, &gin.Context{Request: &http.Request{}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Empty(s.T(), assessmentID)
		})

		s.Run("request Recaptcha-Token header incorrect length", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(nil, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123", "456"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Empty(s.T(), assessmentID)
		})

		s.Run("captcha assessment error", func() {
			s.OverrideApplicationDefault(
				testconfig.RegistrationService().
					Verification().Enabled(true).
					Verification().CaptchaEnabled(true))

			isVerificationRequired, score, assessmentID := service.IsPhoneVerificationRequired(&FakeCaptchaChecker{result: fmt.Errorf("assessment failed")}, &gin.Context{Request: &http.Request{Header: http.Header{"Recaptcha-Token": []string{"123"}}}})
			assert.True(s.T(), isVerificationRequired)
			assert.InDelta(s.T(), float32(-1), score, 0.01)
			assert.Empty(s.T(), assessmentID)
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
			assert.Empty(s.T(), assessmentID)
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
			assert.Empty(s.T(), assessmentID)
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

	// Create a new UserSignup
	username, userSignup := s.newUserSignupComplete()

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

		_, err := application.SignupService().GetSignup(c, username, true)
		require.NoError(s.T(), err)

		modified := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), modified)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "cocochanel", modified.Spec.IdentityClaims.PreferredUsername)
		require.Equal(s.T(), "Foo", modified.Spec.IdentityClaims.GivenName)
		require.Equal(s.T(), "Bar", modified.Spec.IdentityClaims.FamilyName)
		require.Equal(s.T(), "Red Hat", modified.Spec.IdentityClaims.Company)

		require.Equal(s.T(), "5647382910", modified.Spec.IdentityClaims.AccountID)
		require.Equal(s.T(), "UserID123", modified.Spec.IdentityClaims.Sub)
		require.Equal(s.T(), "0192837465", modified.Spec.IdentityClaims.UserID)
		require.Equal(s.T(), "original-sub-value", modified.Spec.IdentityClaims.OriginalSub)
		require.Equal(s.T(), "foo@redhat.com", modified.Spec.IdentityClaims.Email)
		require.Equal(s.T(), "fd2addbd8d82f0d2dc088fa122377eaa", modified.Labels["toolchain.dev.openshift.com/email-hash"])
		require.Equal(s.T(), "4242", modified.Spec.IdentityClaims.AccountNumber)

		s.Run("GivenName property updated when set in context", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set(context.GivenNameKey, "Jonathan")

			_, err := application.SignupService().GetSignup(c, username, true)
			require.NoError(s.T(), err)

			modified := &toolchainv1alpha1.UserSignup{}
			err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), modified)
			require.NoError(s.T(), err)

			require.Equal(s.T(), "Jonathan", modified.Spec.IdentityClaims.GivenName)

			// Confirm that some other properties were not changed
			require.Equal(s.T(), "cocochanel", modified.Spec.IdentityClaims.PreferredUsername)
			require.Equal(s.T(), "Bar", modified.Spec.IdentityClaims.FamilyName)
			require.Equal(s.T(), "Red Hat", modified.Spec.IdentityClaims.Company)

			require.Equal(s.T(), "5647382910", modified.Spec.IdentityClaims.AccountID)
			require.Equal(s.T(), "UserID123", modified.Spec.IdentityClaims.Sub)
			require.Equal(s.T(), "0192837465", modified.Spec.IdentityClaims.UserID)
			require.Equal(s.T(), "original-sub-value", modified.Spec.IdentityClaims.OriginalSub)
			require.Equal(s.T(), "foo@redhat.com", modified.Spec.IdentityClaims.Email)
			require.Equal(s.T(), "fd2addbd8d82f0d2dc088fa122377eaa", modified.Labels["toolchain.dev.openshift.com/email-hash"])

			s.Run("FamilyName and Company properties updated when set in context", func() {
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Set(context.FamilyNameKey, "Smythe")
				c.Set(context.CompanyKey, "Red Hat")

				_, err := application.SignupService().GetSignup(c, username, true)
				require.NoError(s.T(), err)

				modified := &toolchainv1alpha1.UserSignup{}
				err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), modified)
				require.NoError(s.T(), err)

				require.Equal(s.T(), "Smythe", modified.Spec.IdentityClaims.FamilyName)
				require.Equal(s.T(), "Red Hat", modified.Spec.IdentityClaims.Company)

				require.Equal(s.T(), "Jonathan", modified.Spec.IdentityClaims.GivenName)
				require.Equal(s.T(), "cocochanel", modified.Spec.IdentityClaims.PreferredUsername)

				require.Equal(s.T(), "5647382910", modified.Spec.IdentityClaims.AccountID)
				require.Equal(s.T(), "UserID123", modified.Spec.IdentityClaims.Sub)
				require.Equal(s.T(), "0192837465", modified.Spec.IdentityClaims.UserID)
				require.Equal(s.T(), "original-sub-value", modified.Spec.IdentityClaims.OriginalSub)
				require.Equal(s.T(), "foo@redhat.com", modified.Spec.IdentityClaims.Email)
				require.Equal(s.T(), "fd2addbd8d82f0d2dc088fa122377eaa", modified.Labels["toolchain.dev.openshift.com/email-hash"])

				s.Run("Remaining properties updated when set in context", func() {
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Set(context.SubKey, "987654321")
					c.Set(context.UserIDKey, "123456777")
					c.Set(context.AccountIDKey, "777654321")
					c.Set(context.OriginalSubKey, "jsmythe-original-sub")
					c.Set(context.EmailKey, "jsmythe@redhat.com")
					c.Set(context.AccountNumberKey, "123456789")

					_, err := application.SignupService().GetSignup(c, username, true)
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
					require.Equal(s.T(), "123456789", modified.Spec.IdentityClaims.AccountNumber) // check that account number was set
				})
			})
		})
	})
}

func (s *TestSignupServiceSuite) newUserSignupComplete() (string, *toolchainv1alpha1.UserSignup) {
	return "ted@kubesaw", testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("ted@kubesaw"),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupUserEmailHashLabelKey, "90cb861692508c36933b85dfe43f5369"),
		testusersignup.SignupComplete(""),
		testusersignup.ApprovedAutomaticallyAgo(time.Second),
		testusersignup.WithCompliantUsername("ted"),
		testusersignup.WithHomeSpace("ted"),
		testusersignup.WithScheduledDeactivationTimestamp(util.Ptr(v1.NewTime(time.Now().Add(30*time.Hour*24)))))
}

func (s *TestSignupServiceSuite) newProvisionedMUR(name string) *toolchainv1alpha1.MasterUserRecord {
	return masteruserrecord.NewMasterUserRecord(s.T(), name,
		masteruserrecord.ProvisionedMur(util.Ptr(v1.NewTime(time.Now()))),
		masteruserrecord.StatusCondition(toolchainv1alpha1.Condition{
			Type:    toolchainv1alpha1.MasterUserRecordReady,
			Status:  apiv1.ConditionTrue,
			Reason:  "mur_ready_reason",
			Message: "mur_ready_message",
		}),
		masteruserrecord.StatusUserAccount("member-123"))
}

func (s *TestSignupServiceSuite) newSpace(name string) *toolchainv1alpha1.Space {
	return space.NewSpace(commontest.HostOperatorNs, name,
		space.WithLabel(toolchainv1alpha1.SpaceCreatorLabelKey, name),
		space.WithSpecTargetCluster("member-123"),
		space.WithSpecTargetClusterRoles([]string{"cluster-role.toolchain.dev.openshift.com/tenant"}),
		space.WithTierName("base1ns"),
		space.WithStatusTargetCluster("member-123"),
		space.WithStatusProvisionedNamespaces([]toolchainv1alpha1.SpaceNamespace{
			{
				Name: fmt.Sprintf("%s-dev", name),
				Type: "default",
			},
		}))
}

func (s *TestSignupServiceSuite) newSpaceBinding(murName, spaceName string) *toolchainv1alpha1.SpaceBinding {
	name, err := uuid.NewV4()
	require.NoError(s.T(), err)

	return fake.NewSpaceBinding(name.String(), murName, spaceName, "admin")
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
