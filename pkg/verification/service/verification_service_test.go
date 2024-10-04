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

	senderpkg "github.com/codeready-toolchain/registration-service/pkg/verification/sender"
	testutil "github.com/codeready-toolchain/registration-service/test/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
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

func httpClientFactoryOption() func(serviceFactory *factory.ServiceFactory) {

	httpClient := &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(httpClient)

	serviceOption := func(svc *verificationservice.ServiceImpl) {
		svc.HTTPClient = httpClient
	}

	opt := func(serviceFactory *factory.ServiceFactory) {
		serviceFactory.WithVerificationServiceOption(serviceOption)
	}

	return opt
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

	userSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "sbryzak@redhat.com",
			},
		},
	}

	// Create a second UserSignup which we will test by username lookup instead of UserID lookup.  This will also function
	// as some additional noise for the test
	userSignup2 := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jsmith",
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+61NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "jsmith",
			},
		},
	}

	// Require verification for both UserSignups
	states.SetVerificationRequired(userSignup, true)
	states.SetVerificationRequired(userSignup2, true)

	// Add both UserSignups to the fake client
	fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup, userSignup2)

	// Test the init verification for the first UserSignup
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
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
	// This time we won't pass in the UserID, just the username yet still expect the UserSignup to be found
	err = application.VerificationService().InitVerification(ctx, "", userSignup2.Spec.IdentityClaims.PreferredUsername, "+61NUMBER", "1")
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

	userSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "shane@redhat.com",
			},
		},
	}

	states.SetVerificationRequired(userSignup, true)

	s.T().Run("when client GET call fails should return error", func(t *testing.T) {
		fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup)

		// Cause the client GET call to fail
		fakeClient.MockGet = func(ctx gocontext.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok {
				return errors.New("get failed")
			}
			return fakeClient.Client.Get(ctx, key, obj, opts...)
		}

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
		require.EqualError(t, err, "get failed: error retrieving usersignup: 123", err.Error())
	})

	s.T().Run("when client UPDATE call fails indefinitely should return error", func(t *testing.T) {
		fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup)
		fakeClient.MockUpdate = func(ctx gocontext.Context, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*toolchainv1alpha1.UserSignup); ok {
				return errors.New("there was an error while updating your account - please wait a moment before trying again. If this error persists, please contact the Developer Sandbox team at devsandbox@redhat.com \"+\n\t\t\t\"for assistance: error while verifying phone code")
			}
			return fakeClient.Client.Update(ctx, obj, opts...)
		}

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
		require.EqualError(t, err, "there was an error while updating your account - please wait a moment before "+
			"trying again. If this error persists, please contact the Developer Sandbox team at devsandbox@redhat.com "+
			"for assistance: error while verifying phone code")
	})

	s.T().Run("when client UPDATE call fails twice should return ok", func(t *testing.T) {
		fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup)

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
		err := application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
		require.NoError(t, err)

		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)

		require.NotEmpty(t, signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(reqBody)
		require.NoError(t, err)
		reqValue := buf.String()

		params, err := url.ParseQuery(reqValue)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
			signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
			params.Get("Body"))
		require.Equal(t, "CodeReady", params.Get("From"))
		require.Equal(t, "+1NUMBER", params.Get("To"))
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

	userSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Add(-25 * time.Hour).Format(verificationservice.TimestampLayout),
				toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:            "3",
				toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey:          "123456",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "shane@redhat.com",
			},
		},
	}
	states.SetVerificationRequired(userSignup, true)

	fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
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

	userSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey:       "abc",
				toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Format(verificationservice.TimestampLayout),
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "shane@redhat.com",
			},
		},
	}
	states.SetVerificationRequired(userSignup, true)

	_, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
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

	userSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey:       strconv.Itoa(cfg.Verification().DailyLimit()),
				toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Format(verificationservice.TimestampLayout),
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "shane@redhat.com",
			},
		},
	}
	states.SetVerificationRequired(userSignup, true)

	_, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "+1NUMBER", "1")
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

	alphaUserSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha",
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: phoneHash,
				toolchainv1alpha1.UserSignupStateLabelKey:         toolchainv1alpha1.UserSignupStateLabelValueApproved,
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "alpha@foxtrot.com",
			},
		},
	}
	states.SetApprovedManually(alphaUserSignup, true)

	bravoUserSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bravo",
			Namespace: configuration.Namespace(),
			Labels:    map[string]string{},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "bravo@foxtrot.com",
			},
		},
	}
	states.SetVerificationRequired(bravoUserSignup, true)

	fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), alphaUserSignup, bravoUserSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, bravoUserSignup.Name, bravoUserSignup.Spec.IdentityClaims.PreferredUsername, e164PhoneNumber, "1")
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

	alphaUserSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha",
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: phoneHash,
				toolchainv1alpha1.UserSignupStateLabelKey:         toolchainv1alpha1.UserSignupStateLabelValueDeactivated,
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "alpha@foxtrot.com",
			},
		},
	}
	states.SetApprovedManually(alphaUserSignup, true)
	states.SetDeactivated(alphaUserSignup, true)

	bravoUserSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bravo",
			Namespace: configuration.Namespace(),
			Labels:    map[string]string{},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PreferredUsername: "bravo@foxtrot.com",
			},
		},
	}
	states.SetVerificationRequired(bravoUserSignup, true)

	fakeClient, application := testutil.PrepareInClusterAppWithOption(s.T(), httpClientFactoryOption(), alphaUserSignup, bravoUserSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := application.VerificationService().InitVerification(ctx, bravoUserSignup.Name, bravoUserSignup.Spec.IdentityClaims.PreferredUsername, e164PhoneNumber, "1")
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

	s.T().Run("verification ok", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey:     "0.8",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "shane@redhat.com",
				},
			},
		}
		states.SetVerificationRequired(userSignup, true)

		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.NoError(t, err)

		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)

		require.False(t, states.VerificationRequired(signup))
	})

	s.T().Run("verification ok for usersignup with username identifier", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "employee085",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey:     "0.7",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "654321",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "employee085@redhat.com",
				},
			},
		}
		states.SetVerificationRequired(userSignup, true)

		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, "", "employee085", "654321")
		require.NoError(t, err)

		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)

		require.False(t, states.VerificationRequired(signup))
	})

	s.T().Run("when verification code is invalid", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "000000",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "shane@redhat.com",
				},
			},
		}

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.Error(t, err)
		e := &crterrors.Error{}
		require.ErrorAs(t, err, &e)
		require.Equal(t, "invalid code: the provided code is invalid", e.Error())
		require.Equal(t, http.StatusForbidden, int(e.Code))
	})

	s.T().Run("when verification code has expired", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(-10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "shane@redhat.com",
				},
			},
		}

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		e := &crterrors.Error{}
		require.ErrorAs(t, err, &e)
		require.Equal(t, "expired: verification code expired", e.Error())
		require.Equal(t, http.StatusForbidden, int(e.Code))
	})

	s.T().Run("when verifications exceeded maximum attempts", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "3",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "shane@redhat.com",
				},
			},
		}

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.EqualError(t, err, "too many verification attempts", err.Error())
	})

	s.T().Run("when verifications attempts has invalid value", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "ABC",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "shane@redhat.com",
				},
			},
		}

		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.EqualError(t, err, "too many verification attempts", err.Error())

		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)

		require.Equal(t, "3", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey])
	})

	s.T().Run("when verifications expiry is corrupt", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     "ABC",
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
					PreferredUsername: "shane@redhat.com",
				},
			},
		}

		_, application := testutil.PrepareInClusterApp(s.T(), userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")
		require.EqualError(t, err, "parsing time \"ABC\" as \"2006-01-02T15:04:05.000Z07:00\": cannot parse \"ABC\" as \"2006\": error parsing expiry timestamp", err.Error())
	})

	s.T().Run("captcha configuration ", func(t *testing.T) {
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
			t.Run(k, func(t *testing.T) {
				// when
				s.OverrideApplicationDefault(
					testconfig.RegistrationService().Verification().CaptchaRequiredScore("0.6"),
					testconfig.RegistrationService().Verification().CaptchaAllowLowScoreReactivation(tc.allowLowScoreReactivationConfiguration),
				)
				userSignup := &toolchainv1alpha1.UserSignup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "123",
						Namespace: configuration.Namespace(),
						Annotations: map[string]string{
							toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
							toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
							toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Minute).Format(verificationservice.TimestampLayout),
						},
						Labels: map[string]string{
							toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
						},
					},
					Spec: toolchainv1alpha1.UserSignupSpec{
						IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
							PreferredUsername: "shane@redhat.com",
						},
					},
				}
				if tc.activationCounterAnnotationValue != "" {
					userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey] = tc.activationCounterAnnotationValue
				}
				if tc.captchaScoreAnnotationValue != "" {
					userSignup.Annotations[toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey] = tc.captchaScoreAnnotationValue
				}
				states.SetVerificationRequired(userSignup, true)

				fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				err := application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "123456")

				// then
				signup := &toolchainv1alpha1.UserSignup{}
				require.NoError(s.T(), fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup))
				if tc.expectedErr != "" {
					require.EqualError(t, err, tc.expectedErr)
				} else {
					require.NoError(t, err)
					require.False(t, states.VerificationRequired(signup))
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

	s.T().Run("verification ok", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second)) // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithTargetCluster(targetCluster))
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.NoError(t, err)
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.False(t, states.VerificationRequired(signup))
		assert.Equal(t, targetCluster, signup.Spec.TargetCluster)
	})

	s.T().Run("last user to signup", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second))                                                                          // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithActivationCount(9), testsocialevent.WithTargetCluster(targetCluster)) // one seat left
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.NoError(t, err)
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.False(t, states.VerificationRequired(signup))
		assert.Equal(t, targetCluster, signup.Spec.TargetCluster)
	})

	s.T().Run("when too many attempts made", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(
			testusersignup.VerificationRequired(time.Second), // just signed up
			testusersignup.WithVerificationAttempts(cfg.Verification().AttemptsAllowed()))
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithTargetCluster(targetCluster))
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.EqualError(t, err, "too many verification attempts: 3")
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.True(t, states.VerificationRequired(signup)) // unchanged
		assert.Empty(t, signup.Spec.TargetCluster)
	})

	s.T().Run("when invalid code", func(t *testing.T) {

		t.Run("first attempt", func(t *testing.T) {
			// given
			userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second)) // just signed up
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

			// when
			err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "invalid")

			// then
			require.EqualError(t, err, "invalid code: the provided code is invalid")
			signup := &toolchainv1alpha1.UserSignup{}
			err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
			require.NoError(s.T(), err)
			require.True(t, states.VerificationRequired(signup))                                              // unchanged
			assert.Equal(t, "1", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		})

		t.Run("second attempt", func(t *testing.T) {
			// given
			userSignup := testusersignup.NewUserSignup(
				testusersignup.VerificationRequired(time.Second), // just signed up
				testusersignup.WithVerificationAttempts(2))       // already tried twice before
			fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup)

			// when
			err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, "invalid")

			// then
			require.EqualError(t, err, "invalid code: the provided code is invalid")
			signup := &toolchainv1alpha1.UserSignup{}
			err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
			require.NoError(s.T(), err)
			require.True(t, states.VerificationRequired(signup))                                              // unchanged
			assert.Equal(t, "3", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		})
	})

	s.T().Run("when max attendees reached", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second))                                                                           // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithActivationCount(10), testsocialevent.WithTargetCluster(targetCluster)) // same as default `spec.MaxAttendees`
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.EqualError(t, err, "invalid code: the event is full")
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.True(t, states.VerificationRequired(signup))
		assert.Equal(t, "1", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		assert.Empty(t, signup.Spec.TargetCluster)
	})

	s.T().Run("when event not open yet", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second))                                                                                            // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithStartTime(time.Now().Add(time.Hour)), testsocialevent.WithTargetCluster(targetCluster)) // starting in 1hr
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.EqualError(t, err, "invalid code: the provided code is invalid")
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.True(t, states.VerificationRequired(signup))
		assert.Equal(t, "1", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		assert.Empty(t, signup.Spec.TargetCluster)
	})

	s.T().Run("when event already closed", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second))                                                                                           // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithEndTime(time.Now().Add(-time.Hour)), testsocialevent.WithTargetCluster(targetCluster)) // ended 1hr ago
		fakeClient, application := testutil.PrepareInClusterApp(s.T(), userSignup, event)

		// when
		err := application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.IdentityClaims.PreferredUsername, event.Name)

		// then
		require.EqualError(t, err, "invalid code: the provided code is invalid")
		signup := &toolchainv1alpha1.UserSignup{}
		err = fakeClient.Get(gocontext.TODO(), client.ObjectKeyFromObject(userSignup), signup)
		require.NoError(s.T(), err)
		require.True(t, states.VerificationRequired(signup))
		assert.Equal(t, "1", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		assert.Empty(t, signup.Spec.TargetCluster)
	})
}
