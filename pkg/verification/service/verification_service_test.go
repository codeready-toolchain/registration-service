package service_test

import (
	"bytes"
	gocontext "context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	senderpkg "github.com/codeready-toolchain/registration-service/pkg/verification/sender"
	testutil "github.com/codeready-toolchain/registration-service/test/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	verificationservice "github.com/codeready-toolchain/registration-service/pkg/verification/service"
	"github.com/codeready-toolchain/registration-service/test"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"
	testsocialevent "github.com/codeready-toolchain/toolchain-common/pkg/test/socialevent"
	testusersignup "github.com/codeready-toolchain/toolchain-common/pkg/test/usersignup"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testSecretName = "host-operator-secret"

	twilioSIDKey        = "twilio.sid"
	twilioTokenKey      = "twilio.token" // nolint:gosec
	twilioFromNumberKey = "twilio.fromnumber"

	awsAccessKeyIDKey  = "aws.accesskeyid"
	awsSecretAccessKey = "aws.secretaccesskey"
)

type TestVerificationServiceSuite struct {
	test.UnitTestSuite
	httpClient *http.Client
}

func TestRunVerificationServiceSuite(t *testing.T) {
	suite.Run(t, &TestVerificationServiceSuite{test.UnitTestSuite{}, nil})
}

func (s *TestVerificationServiceSuite) ServiceConfiguration(accountSID, authToken, fromNumber string) {

	ns, err := commonconfig.GetWatchNamespace()
	require.NoError(s.T(), err)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSecretName,
			Namespace: ns,
		},
		Data: map[string][]byte{
			twilioSIDKey:        []byte(accountSID),
			twilioTokenKey:      []byte(authToken),
			twilioFromNumberKey: []byte(fromNumber),
			// Set the following two values to manually test with AWS
			awsAccessKeyIDKey:  []byte(""),
			awsSecretAccessKey: []byte(""),
		},
	}

	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Namespace(ns).
			Verification().AttemptsAllowed(3).
			Verification().DailyLimit(3).
			Verification().CodeExpiresInMin(5).
			// Override this to manually test with AWS - set to "aws"
			Verification().NotificationSender("").
			Verification().AWSRegion("us-west-2").
			Verification().AWSSenderID("Sandbox").
			Verification().Secret().
			Ref(testSecretName).
			TwilioAccountSID(twilioSIDKey).
			TwilioAuthToken(twilioTokenKey).
			TwilioFromNumber(twilioFromNumberKey).
			AWSAccessKeyID(awsAccessKeyIDKey).
			AWSSecretAccessKey(awsSecretAccessKey))

	s.SetSecret(secret)
}

func (s *TestVerificationServiceSuite) TestInitVerification() {
	s.ServiceConfiguration("xxx", "yyy", "CodeReady")

	defer gock.Off()

	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")

	var reqBody io.ReadCloser
	obs := func(request *http.Request, _ gock.Mock) {
		reqBody = request.Body
		defer request.Body.Close()
	}

	gock.Observe(obs)

	userSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("johnny@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
		testusersignup.VerificationRequiredAgo(time.Second))

	// Create a second UserSignup as some additional noise for the test
	userSignup2 := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("jsmith@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+61NUMBER"),
		testusersignup.VerificationRequiredAgo(time.Second))

	// Add both UserSignups to the fake client
	fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, userSignup2)

	// Test the init verification for the first UserSignup
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, "johnny@kubesaw", "+1NUMBER", "1")
	require.NoError(s.T(), err)

	signup := &toolchainv1alpha1.UserSignup{}
	err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
	require.NoError(s.T(), err)

	// Ensure the verification code is set
	require.NotEmpty(s.T(), signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(s.T(), err)
	reqValue := buf.String()

	params, err := url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
		signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+1NUMBER", params.Get("To"))

	// Test the init verification for the second UserSignup - Setup gock again for another request
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")

	obs = func(request *http.Request, _ gock.Mock) {
		reqBody = request.Body
		defer request.Body.Close()
	}
	gock.Observe(obs)

	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	// for the second usersignup
	err = application.VerificationService().InitVerification(ctx, "jsmith@kubesaw", "+61NUMBER", "1")
	require.NoError(s.T(), err)

	signup2 := &toolchainv1alpha1.UserSignup{}
	err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup2), signup2)
	require.NoError(s.T(), err)

	// Ensure the verification code is set
	require.NotEmpty(s.T(), signup2.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf = new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(s.T(), err)
	reqValue = buf.String()

	params, err = url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
		signup2.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+61NUMBER", params.Get("To"))
}

func (s *TestVerificationServiceSuite) TestNotificationSender() {
	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().NotificationSender("aWs"))

	sender := senderpkg.CreateNotificationSender(nil)
	require.IsType(s.T(), &senderpkg.AmazonSNSSender{}, sender)

	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().NotificationSender(""))

	sender = senderpkg.CreateNotificationSender(nil)
	require.IsType(s.T(), &senderpkg.TwilioNotificationSender{}, sender)
}

func (s *TestVerificationServiceSuite) TestInitVerificationClientFailure() {
	s.ServiceConfiguration("xxx", "yyy", "CodeReady")

	defer gock.Off()

	gock.New("https://api.twilio.com").
		Times(2).
		Reply(http.StatusNoContent).
		BodyString("")

	var reqBody io.ReadCloser
	obs := func(request *http.Request, _ gock.Mock) {
		reqBody = request.Body
	}

	gock.Observe(obs)

	userSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("johnny@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
		testusersignup.VerificationRequiredAgo(time.Second))

	s.Run("when client GET call fails should return error", func() {
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		// Cause the client GET call to fail
		fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok {
				return errors.New("get failed")
			}
			return fakeClient.Client.Get(ctx, key, obj, opts...)
		}

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().InitVerification(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
		require.EqualError(s.T(), err, "get failed: error retrieving usersignup with username 'johnny@kubesaw'", err.Error())
	})

	s.Run("when client UPDATE call fails indefinitely should return error", func() {
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)
		fakeClient.MockUpdate = func(ctx gocontext.Context, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok {
				return errors.New("there was an error while updating your account - please wait a moment before trying again. If this error persists, please contact the Developer Sandbox team at devsandbox@redhat.com \"+\n\t\t\t\"for assistance: error while verifying phone code")
			}
			return fakeClient.Client.Update(ctx, obj, opts...)
		}

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().InitVerification(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
		require.EqualError(s.T(), err, "there was an error while updating your account - please wait a moment before "+
			"trying again. If this error persists, please contact the Developer Sandbox team at devsandbox@redhat.com "+
			"for assistance: error while verifying phone code")
	})

	s.Run("when client UPDATE call fails twice should return ok", func() {
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		failCount := 0
		// Cause the client UPDATE call to fail just twice
		fakeClient.MockUpdate = func(ctx gocontext.Context, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok && failCount < 2 {
				failCount++
				return errors.New("update failed")
			}
			return fakeClient.Client.Update(ctx, obj, opts...)
		}

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().InitVerification(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
		require.NoError(s.T(), err)

		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(reqBody)
		require.NoError(s.T(), err)
		reqValue := buf.String()

		params, err := url.ParseQuery(reqValue)
		require.NoError(s.T(), err)
		require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
			signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
			params.Get("Body"))
		require.Equal(s.T(), "CodeReady", params.Get("From"))
		require.Equal(s.T(), "+1NUMBER", params.Get("To"))
	})
}

func (s *TestVerificationServiceSuite) TestInitVerificationPassesWhenMaxCountReachedAndTimestampElapsed() {
	// Setup gock to intercept calls made to the Twilio API
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	s.ServiceConfiguration("xxx", "yyy", "CodeReady")

	now := time.Now()

	var reqBody io.ReadCloser
	obs := func(request *http.Request, _ gock.Mock) {
		reqBody = request.Body
	}

	gock.Observe(obs)

	userSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("johny@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey, now.Add(-25*time.Hour).Format(verificationservice.TimestampLayout)),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey, "3"),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "123456"),
		testusersignup.VerificationRequiredAgo(time.Second))

	fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
	require.NoError(s.T(), err)

	signup := &toolchainv1alpha1.UserSignup{}
	err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(s.T(), err)
	reqValue := buf.String()

	params, err := url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
		signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+1NUMBER", params.Get("To"))
	require.Equal(s.T(), "1", signup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsWhenCountContainsInvalidValue() {
	defer gock.Off()
	// call override config to ensure the factory option takes effect
	s.OverrideApplicationDefault()

	now := time.Now()

	userSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("johny@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey, "abc"),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey, now.Format(verificationservice.TimestampLayout)),
		testusersignup.VerificationRequiredAgo(time.Second))

	_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
	require.EqualError(s.T(), err, "daily limit exceeded: cannot generate new verification code")
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsDailyCounterExceeded() {
	// Setup gock to intercept calls made to the Twilio API
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	// call override config to ensure the factory option takes effect
	s.OverrideApplicationDefault()
	cfg := configuration.GetRegistrationServiceConfig()

	now := time.Now()

	userSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("johny@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey, strconv.Itoa(cfg.Verification().DailyLimit())),
		testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey, now.Format(verificationservice.TimestampLayout)),
		testusersignup.VerificationRequiredAgo(time.Second))

	_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
	require.EqualError(s.T(), err, "daily limit exceeded: cannot generate new verification code", err.Error())
	require.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsWhenPhoneNumberInUse() {
	// Setup gock to intercept calls made to the Twilio API
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	// call override config to ensure the factory option takes effect
	s.OverrideApplicationDefault()

	e164PhoneNumber := "+19875551122"

	// calculate the phone number hash
	phoneHash := hash.EncodeString(e164PhoneNumber)

	alphaUserSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("alpha@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, phoneHash),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupStateLabelKey, toolchainv1alpha1.UserSignupStateLabelValueApproved),
		testusersignup.ApprovedManually())

	bravoUserSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("bravo@kubesaw"),
		testusersignup.VerificationRequiredAgo(time.Second))

	fakeClient, application := testutil.PrepareInClusterApp(s.T(), alphaUserSignup, bravoUserSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, bravoUserSignup.Spec.IdentityClaims.PreferredUsername, e164PhoneNumber, "1")
	require.Error(s.T(), err)
	require.Equal(s.T(), "phone number already in use: cannot register using phone number: +19875551122", err.Error())

	// Reload bravoUserSignup
	signup := &toolchainv1alpha1.UserSignup{}
	err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(bravoUserSignup), signup)
	require.NoError(s.T(), err)

	require.Empty(s.T(), signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationOKWhenPhoneNumberInUseByDeactivatedUserSignup() {
	// Setup gock to intercept calls made to the Twilio API
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	// call override config to ensure the factory option takes effect
	s.OverrideApplicationDefault()

	e164PhoneNumber := "+19875553344"

	// calculate the phone number hash
	phoneHash := hash.EncodeString(e164PhoneNumber)

	alphaUserSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("alpha@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, phoneHash),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupStateLabelKey, toolchainv1alpha1.UserSignupStateLabelValueDeactivated),
		testusersignup.ApprovedManually(),
		testusersignup.Deactivated())

	bravoUserSignup := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("bravo@kubesaw"),
		testusersignup.VerificationRequiredAgo(time.Second))

	fakeClient, application := testutil.PrepareInClusterApp(s.T(), alphaUserSignup, bravoUserSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, bravoUserSignup.Spec.IdentityClaims.PreferredUsername, e164PhoneNumber, "1")
	require.NoError(s.T(), err)

	// Reload bravoUserSignup
	signup := &toolchainv1alpha1.UserSignup{}
	err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(bravoUserSignup), signup)
	require.NoError(s.T(), err)

	// Just confirm that verification has been initialized by testing whether a verification code has been set
	require.NotEmpty(s.T(), signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestVerifyPhoneCode() {
	// given
	now := time.Now()

	s.Run("verification ok", func() {

		userSignup := testusersignup.NewUserSignup(
			testusersignup.WithEncodedName("johny@kubesaw"),
			testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationAttemptsAnnotationKey, "0"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey, "0.8"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "123456"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationExpiryAnnotationKey, now.Add(10*time.Second).Format(verificationservice.TimestampLayout)),
			testusersignup.VerificationRequiredAgo(time.Second))

		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.NoError(s.T(), err)

		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)

		require.False(s.T(), states.VerificationRequired(signup))
	})

	s.Run("verification ok for usersignup with username identifier", func() {

		userSignup := testusersignup.NewUserSignup(
			testusersignup.WithEncodedName("employee085@kubesaw"),
			testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationAttemptsAnnotationKey, "0"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey, "0.7"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "654321"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationExpiryAnnotationKey, now.Add(10*time.Second).Format(verificationservice.TimestampLayout)),
			testusersignup.VerificationRequiredAgo(time.Second))

		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, "employee085@kubesaw", "654321")
		require.NoError(s.T(), err)

		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)

		require.False(s.T(), states.VerificationRequired(signup))
	})

	s.Run("when verification code is invalid", func() {

		userSignup := testusersignup.NewUserSignup(
			testusersignup.WithEncodedName("johny@kubesaw"),
			testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationAttemptsAnnotationKey, "0"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "000000"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationExpiryAnnotationKey, now.Add(10*time.Second).Format(verificationservice.TimestampLayout)),
		)

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.Error(s.T(), err)
		e := &crterrors.Error{}
		require.ErrorAs(s.T(), err, &e)
		require.Equal(s.T(), "invalid code: the provided code is invalid", e.Error())
		require.Equal(s.T(), http.StatusForbidden, int(e.Code))
	})

	s.Run("when verification code has expired", func() {

		userSignup := testusersignup.NewUserSignup(
			testusersignup.WithEncodedName("johny@kubesaw"),
			testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationAttemptsAnnotationKey, "0"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "123456"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationExpiryAnnotationKey, now.Add(-10*time.Second).Format(verificationservice.TimestampLayout)),
		)

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		e := &crterrors.Error{}
		require.ErrorAs(s.T(), err, &e)
		require.Equal(s.T(), "expired: verification code expired", e.Error())
		require.Equal(s.T(), http.StatusForbidden, int(e.Code))
	})

	s.Run("when verifications exceeded maximum attempts", func() {

		userSignup := testusersignup.NewUserSignup(
			testusersignup.WithEncodedName("johny@kubesaw"),
			testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationAttemptsAnnotationKey, "3"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "123456"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationExpiryAnnotationKey, now.Add(10*time.Second).Format(verificationservice.TimestampLayout)),
		)

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.EqualError(s.T(), err, "too many verification attempts", err.Error())
	})

	s.Run("when verifications attempts has invalid value", func() {

		userSignup := testusersignup.NewUserSignup(
			testusersignup.WithEncodedName("johny@kubesaw"),
			testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationAttemptsAnnotationKey, "ABC"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "123456"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationExpiryAnnotationKey, now.Add(10*time.Second).Format(verificationservice.TimestampLayout)),
		)

		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.EqualError(s.T(), err, "too many verification attempts", err.Error())

		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "3", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey])
	})

	s.Run("when verifications expiry is corrupt", func() {

		userSignup := testusersignup.NewUserSignup(
			testusersignup.WithEncodedName("johny@kubesaw"),
			testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationAttemptsAnnotationKey, "0"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "123456"),
			testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationExpiryAnnotationKey, "ABC"),
		)

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.EqualError(s.T(), err, "parsing time \"ABC\" as \"2006-01-02T15:04:05.000Z07:00\": cannot parse \"ABC\" as \"2006\": error parsing expiry timestamp", err.Error())
	})

	s.Run("captcha configuration ", func() {
		tests := map[string]struct {
			activationCounterAnnotationValue       string
			captchaScoreAnnotationValue            string
			allowLowScoreReactivationConfiguration bool
			expectedErr                            string
		}{
			"captcha score below required score but it's a reactivation": {
				activationCounterAnnotationValue:       "2",   // user is reactivating
				captchaScoreAnnotationValue:            "0.5", // and captcha score is low
				allowLowScoreReactivationConfiguration: true,
			},
			"captcha score below required score but it's not a reactivation": {
				activationCounterAnnotationValue:       "1",   // first time user
				captchaScoreAnnotationValue:            "0.5", // and captcha score is low
				allowLowScoreReactivationConfiguration: true,
				expectedErr:                            "verification failed: verification is not available at this time",
			},
			"activation counter is invalid and captcha score is low": {
				activationCounterAnnotationValue:       "x",   // something wrong happened
				captchaScoreAnnotationValue:            "0.5", // and captcha score is low
				allowLowScoreReactivationConfiguration: true,
				expectedErr:                            "verification failed: verification is not available at this time",
			},
			"activation counter is invalid and captcha score is ok": {
				activationCounterAnnotationValue:       "x",   // something wrong happened
				captchaScoreAnnotationValue:            "0.6", // but captcha score is ok
				allowLowScoreReactivationConfiguration: true,
			},
			"allow low score reactivation disabled - captcha score below required score and it's a reactivation": {
				activationCounterAnnotationValue:       "2",   // user is reactivating
				captchaScoreAnnotationValue:            "0.5", //  captcha score is low
				allowLowScoreReactivationConfiguration: false,
				expectedErr:                            "verification failed: verification is not available at this time",
			},
			"allow low score reactivation disabled - captcha score below required score and it's not a reactivation": {
				activationCounterAnnotationValue:       "1",   // first time user
				captchaScoreAnnotationValue:            "0.5", //  captcha score is low
				allowLowScoreReactivationConfiguration: false,
				expectedErr:                            "verification failed: verification is not available at this time",
			},
			"allow low score reactivation disabled - captcha score ok": {
				activationCounterAnnotationValue:       "1",   // first time user
				captchaScoreAnnotationValue:            "0.6", //  captcha score is ok
				allowLowScoreReactivationConfiguration: false,
			},
			"no score annotation": {
				activationCounterAnnotationValue:       "1", // first time user
				captchaScoreAnnotationValue:            "",  // score annotation is missing
				allowLowScoreReactivationConfiguration: true,
				// no error is expected in this case and the verification should proceed
			},
			"score annotation is invalid": {
				activationCounterAnnotationValue:       "1",   // first time user
				captchaScoreAnnotationValue:            "xxx", // score annotation is invalid
				allowLowScoreReactivationConfiguration: true,
				// no error is expected in this case and the verification should proceed
			},
			"no activation counter annotation and low captcha score": {
				activationCounterAnnotationValue:       "",    // activation counter is missing thus required score will be compared with captcha score
				captchaScoreAnnotationValue:            "0.5", // score is low thus verification will fail
				allowLowScoreReactivationConfiguration: true,
				expectedErr:                            "verification failed: verification is not available at this time",
			},
			"no activation counter annotation and captcha score ok": {
				activationCounterAnnotationValue:       "",    // activation counter is missing thus required score will be compared with captcha score
				captchaScoreAnnotationValue:            "0.6", // score is ok thus verification will succeed
				allowLowScoreReactivationConfiguration: true,
			},
		}
		for k, tc := range tests {
			s.Run(k, func() {
				// when
				s.OverrideApplicationDefault(
					testconfig.RegistrationService().Verification().CaptchaRequiredScore("0.6"),
					testconfig.RegistrationService().Verification().CaptchaAllowLowScoreReactivation(tc.allowLowScoreReactivationConfiguration),
				)

				userSignup := testusersignup.NewUserSignup(
					testusersignup.WithEncodedName("johny@kubesaw"),
					testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "+1NUMBER"),
					testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationAttemptsAnnotationKey, "0"),
					testusersignup.WithAnnotation(toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey, "123456"),
					testusersignup.WithAnnotation(toolchainv1alpha1.UserVerificationExpiryAnnotationKey, now.Add(10*time.Second).Format(verificationservice.TimestampLayout)),
					testusersignup.VerificationRequiredAgo(time.Second))
				if tc.activationCounterAnnotationValue != "" {
					userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey] = tc.activationCounterAnnotationValue
				}
				if tc.captchaScoreAnnotationValue != "" {
					userSignup.Annotations[toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey] = tc.captchaScoreAnnotationValue
				}

				fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")

				// then
				signup := &toolchainv1alpha1.UserSignup{}
				require.NoError(s.T(), fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup))
				if tc.expectedErr != "" {
					require.EqualError(s.T(), err, tc.expectedErr)
				} else {
					require.NoError(s.T(), err)
					require.False(s.T(), states.VerificationRequired(signup))
				}
			})
		}
	})
}

func (s *TestVerificationServiceSuite) TestVerifyActivationCode() {
	s.testVerifyActivationCode("")
	s.testVerifyActivationCode("member-1")
}

func (s *TestVerificationServiceSuite) testVerifyActivationCode(targetCluster string) {
	// given

	cfg := configuration.GetRegistrationServiceConfig()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	s.Run("verification ok", func() {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second)) // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithTargetCluster(targetCluster))
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.NoError(s.T(), err)
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.False(s.T(), states.VerificationRequired(signup))
		assert.Equal(s.T(), targetCluster, signup.Spec.TargetCluster)
	})

	s.Run("last user to signup", func() {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second))                                                                       // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithActivationCount(9), testsocialevent.WithTargetCluster(targetCluster)) // one seat left
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.NoError(s.T(), err)
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.False(s.T(), states.VerificationRequired(signup))
		assert.Equal(s.T(), targetCluster, signup.Spec.TargetCluster)
	})

	s.Run("when too many attempts made", func() {
		// given
		userSignup := testusersignup.NewUserSignup(
			testusersignup.VerificationRequiredAgo(time.Second), // just signed up
			testusersignup.WithVerificationAttempts(cfg.Verification().AttemptsAllowed()))
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithTargetCluster(targetCluster))
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.EqualError(s.T(), err, "too many verification attempts: 3")
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.True(s.T(), states.VerificationRequired(signup)) // unchanged
		assert.Empty(s.T(), signup.Spec.TargetCluster)
	})

	s.Run("when invalid code", func() {

		s.Run("first attempt", func() {
			// given
			userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second)) // just signed up
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

			// when
			err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "invalid")

			// then
			require.EqualError(s.T(), err, "invalid code: the provided code is invalid")
			signup := &toolchainv1alpha1.UserSignup{}
			err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
			require.NoError(s.T(), err)
			require.True(s.T(), states.VerificationRequired(signup))                                              // unchanged
			assert.Equal(s.T(), "1", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		})

		s.Run("second attempt", func() {
			// given
			userSignup := testusersignup.NewUserSignup(
				testusersignup.VerificationRequiredAgo(time.Second), // just signed up
				testusersignup.WithVerificationAttempts(2))          // already tried twice before
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

			// when
			err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, "invalid")

			// then
			require.EqualError(s.T(), err, "invalid code: the provided code is invalid")
			signup := &toolchainv1alpha1.UserSignup{}
			err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
			require.NoError(s.T(), err)
			require.True(s.T(), states.VerificationRequired(signup))                                              // unchanged
			assert.Equal(s.T(), "3", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		})
	})

	s.Run("when max attendees reached", func() {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second))                                                                        // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithActivationCount(10), testsocialevent.WithTargetCluster(targetCluster)) // same as default `spec.MaxAttendees`
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.EqualError(s.T(), err, "invalid code: the event is full")
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.True(s.T(), states.VerificationRequired(signup))
		assert.Equal(s.T(), "1", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		assert.Empty(s.T(), signup.Spec.TargetCluster)
	})

	s.Run("when event not open yet", func() {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second))                                                                                         // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithStartTime(time.Now().Add(time.Hour)), testsocialevent.WithTargetCluster(targetCluster)) // starting in 1hr
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.EqualError(s.T(), err, "invalid code: the provided code is invalid")
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.True(s.T(), states.VerificationRequired(signup))
		assert.Equal(s.T(), "1", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		assert.Empty(s.T(), signup.Spec.TargetCluster)
	})

	s.Run("when event already closed", func() {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequiredAgo(time.Second))                                                                                        // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithEndTime(time.Now().Add(-time.Hour)), testsocialevent.WithTargetCluster(targetCluster)) // ended 1hr ago
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.EqualError(s.T(), err, "invalid code: the provided code is invalid")
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.True(s.T(), states.VerificationRequired(signup))
		assert.Equal(s.T(), "1", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		assert.Empty(s.T(), signup.Spec.TargetCluster)
	})
}

func (s *TestVerificationServiceSuite) TestPhoneNumberAlreadyInUse() {
	bannedUser := &toolchainv1alpha1.BannedUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "banneduser",
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

	userSignupApproved := testusersignup.NewUserSignup(
		testusersignup.WithEncodedName("johnny@kubesaw"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserEmailHashLabelKey, "a7b1b413c1cbddbcd19a51222ef8e20a"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "fd276563a8232d16620da8ec85d0575f"),
		testusersignup.WithLabel(toolchainv1alpha1.UserSignupStateLabelKey, toolchainv1alpha1.UserSignupStateLabelValueApproved))

	s.Run("when phone is not used yet", func() {
		// given
		fakeClient := commontest.NewFakeClient(s.T(), userSignupApproved, bannedUser)
		nsdClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

		// when
		err := verificationservice.PhoneNumberAlreadyInUse(nsdClient, "jsmith", "+987654321")

		// then
		require.NoError(s.T(), err)
	})

	s.Run("when phone is used but not by approved user", func() {
		for _, state := range []string{"", toolchainv1alpha1.StateLabelValuePending, toolchainv1alpha1.UserSignupStateLabelValueDeactivated, toolchainv1alpha1.UserSignupStateLabelValueNotReady} {
			s.Run(fmt.Sprintf("state: %s", state), func() {})
			// given
			userSignup := testusersignup.NewUserSignup(
				testusersignup.WithEncodedName("johnny@kubesaw"),
				testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserEmailHashLabelKey, "a7b1b413c1cbddbcd19a51222ef8e20a"),
				testusersignup.WithLabel(toolchainv1alpha1.UserSignupUserPhoneHashLabelKey, "fd276563a8232d16620da8ec85d0575f"),
				testusersignup.WithLabel(toolchainv1alpha1.UserSignupStateLabelKey, state))

			fakeClient := commontest.NewFakeClient(s.T(), userSignup)
			nsdClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

			// when
			err := verificationservice.PhoneNumberAlreadyInUse(nsdClient, "jsmith", "+987654321")

			// then
			require.NoError(s.T(), err)
		}
	})

	s.Run("when used by banned user", func() {
		// given
		fakeClient := commontest.NewFakeClient(s.T(), bannedUser)
		nsdClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

		// when
		err := verificationservice.PhoneNumberAlreadyInUse(nsdClient, "jsmith", "+12268213044")

		// then
		require.EqualError(s.T(), err, "cannot re-register with phone number: phone number already in use")
	})

	s.Run("when used by another user", func() {
		// given
		fakeClient := commontest.NewFakeClient(s.T(), userSignupApproved)
		nsdClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

		// when
		err := verificationservice.PhoneNumberAlreadyInUse(nsdClient, "jsmith", "+12268213044")

		// then
		require.EqualError(s.T(), err, "cannot re-register with phone number: phone number already in use")
	})

	s.Run("failures", func() {
		s.Run("fails when lists banned users", func() {
			// given
			fakeClient := commontest.NewFakeClient(s.T(), userSignupApproved)
			fakeClient.MockList = func(ctx gocontext.Context, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*toolchainv1alpha1.BannedUserList); ok {
					return fmt.Errorf("list error")
				}
				return fakeClient.Client.List(ctx, list, opts...)
			}
			nsdClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

			// when
			err := verificationservice.PhoneNumberAlreadyInUse(nsdClient, "jsmith", "+12268213044")

			// then
			require.EqualError(s.T(), err, "list error: failed listing banned users")
		})

		s.Run("fails when lists usersignups", func() {
			// given
			fakeClient := commontest.NewFakeClient(s.T(), userSignupApproved)
			fakeClient.MockList = func(ctx gocontext.Context, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*toolchainv1alpha1.UserSignupList); ok {
					return fmt.Errorf("list error")
				}
				return fakeClient.Client.List(ctx, list, opts...)
			}
			nsdClient := namespaced.NewClient(fakeClient, commontest.HostOperatorNs)

			// when
			err := verificationservice.PhoneNumberAlreadyInUse(nsdClient, "jsmith", "+12268213044")

			// then
			require.EqualError(s.T(), err, "list error: failed listing userSignups")
		})
	})

}
