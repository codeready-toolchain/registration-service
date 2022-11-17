package service_test

import (
	"bytes"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
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

func (s *TestVerificationServiceSuite) SetHTTPClientFactoryOption() {

	s.httpClient = &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(s.httpClient)

	serviceOption := func(svc *verificationservice.ServiceImpl) {
		svc.HTTPClient = s.httpClient
	}

	opt := func(serviceFactory *factory.ServiceFactory) {
		serviceFactory.WithVerificationServiceOption(serviceOption)
	}

	s.WithFactoryOption(opt)
}

func (s *TestVerificationServiceSuite) TestInitVerification() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHTTPClientFactoryOption()
	s.ServiceConfiguration("xxx", "yyy", "CodeReady")

	defer gock.Off()

	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")

	var reqBody io.ReadCloser
	obs := func(request *http.Request, mock gock.Mock) {
		reqBody = request.Body
		defer request.Body.Close()
	}

	gock.Observe(obs)

	userSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "sbryzak@redhat.com",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "sbryzak@redhat.com",
		},
	}

	// Create a second UserSignup which we will test by username lookup instead of UserID lookup.  This will also function
	// as some additional noise for the test
	userSignup2 := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jsmith",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "jsmith@redhat.com",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+61NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "jsmith",
		},
	}

	// Require verification for both UserSignups
	states.SetVerificationRequired(userSignup, true)
	states.SetVerificationRequired(userSignup2, true)

	// Add both UserSignups to the fake client
	err := s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	err = s.FakeUserSignupClient.Tracker.Add(userSignup2)
	require.NoError(s.T(), err)

	// Test the init verification for the first UserSignup
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err = s.Application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.Username, "+1NUMBER")
	require.NoError(s.T(), err)

	userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
	require.NoError(s.T(), err)

	// Ensure the verification code is set
	require.NotEmpty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(s.T(), err)
	reqValue := buf.String()

	params, err := url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
		userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+1NUMBER", params.Get("To"))

	// Test the init verification for the second UserSignup - Setup gock again for another request
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")

	obs = func(request *http.Request, mock gock.Mock) {
		reqBody = request.Body
		defer request.Body.Close()
	}
	gock.Observe(obs)

	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	// This time we won't pass in the UserID, just the username yet still expect the UserSignup to be found
	err = s.Application.VerificationService().InitVerification(ctx, "", userSignup2.Spec.Username, "+61NUMBER")
	require.NoError(s.T(), err)

	userSignup2, err = s.FakeUserSignupClient.Get(userSignup2.Name)
	require.NoError(s.T(), err)

	// Ensure the verification code is set
	require.NotEmpty(s.T(), userSignup2.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf = new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(s.T(), err)
	reqValue = buf.String()

	params, err = url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
		userSignup2.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+61NUMBER", params.Get("To"))
}

func (s *TestVerificationServiceSuite) TestNotificationSender() {
	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().NotificationSender("aWs"))

	sender := senderpkg.CreateNotificationSender(nil)
	require.IsType(s.T(), sender, &senderpkg.AmazonSNSSender{})

	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().NotificationSender(""))

	sender = senderpkg.CreateNotificationSender(nil)
	require.IsType(s.T(), sender, &senderpkg.TwilioNotificationSender{})
}

func (s *TestVerificationServiceSuite) TestInitVerificationClientFailure() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHTTPClientFactoryOption()
	s.ServiceConfiguration("xxx", "yyy", "CodeReady")

	defer gock.Off()

	gock.New("https://api.twilio.com").
		Times(2).
		Reply(http.StatusNoContent).
		BodyString("")

	var reqBody io.ReadCloser
	obs := func(request *http.Request, mock gock.Mock) {
		reqBody = request.Body
	}

	gock.Observe(obs)

	userSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "sbryzak@redhat.com",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "sbryzak@redhat.com",
		},
	}

	states.SetVerificationRequired(userSignup, true)

	err := s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	s.T().Run("when client GET call fails should return error", func(t *testing.T) {

		// Cause the client GET call to fail
		s.FakeUserSignupClient.MockGet = func(s string) (*toolchainv1alpha1.UserSignup, error) {
			return nil, errors.New("get failed")
		}
		defer func() { s.FakeUserSignupClient.MockGet = nil }()

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.Username, "+1NUMBER")
		require.EqualError(s.T(), err, "get failed: error retrieving usersignup: 123", err.Error())
	})

	s.T().Run("when client UPDATE call fails indefinitely should return error", func(t *testing.T) {

		// Cause the client UPDATE call to fail always
		s.FakeUserSignupClient.MockUpdate = func(userSignup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
			return nil, errors.New("update failed")
		}
		defer func() { s.FakeUserSignupClient.MockUpdate = nil }()

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.Username, "+1NUMBER")
		require.EqualError(s.T(), err, "there was an error while updating your account - please wait a moment before "+
			"trying again. If this error persists, please contact the Developer Sandbox team at devsandbox@redhat.com "+
			"for assistance: error while verifying phone code")
	})

	s.T().Run("when client UPDATE call fails twice should return ok", func(t *testing.T) {

		failCount := 0

		// Cause the client UPDATE call to fail just twice
		s.FakeUserSignupClient.MockUpdate = func(userSignup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
			if failCount < 2 {
				failCount++
				return nil, errors.New("update failed")
			}
			s.FakeUserSignupClient.MockUpdate = nil
			return s.FakeUserSignupClient.Update(userSignup)
		}
		defer func() { s.FakeUserSignupClient.MockUpdate = nil }()

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.Username, "+1NUMBER")
		require.NoError(s.T(), err)

		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		require.NotEmpty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(reqBody)
		require.NoError(s.T(), err)
		reqValue := buf.String()

		params, err := url.ParseQuery(reqValue)
		require.NoError(s.T(), err)
		require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
			userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
			params.Get("Body"))
		require.Equal(s.T(), "CodeReady", params.Get("From"))
		require.Equal(s.T(), "+1NUMBER", params.Get("To"))
	})
}

func (s *TestVerificationServiceSuite) TestInitVerificationPassesWhenMaxCountReachedAndTimestampElapsed() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHTTPClientFactoryOption()
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	s.ServiceConfiguration("xxx", "yyy", "CodeReady")

	now := time.Now()

	var reqBody io.ReadCloser
	obs := func(request *http.Request, mock gock.Mock) {
		reqBody = request.Body
	}

	gock.Observe(obs)

	userSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey:                 "testuser@redhat.com",
				toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Add(-25 * time.Hour).Format(verificationservice.TimestampLayout),
				toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:            "3",
				toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey:          "123456",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "sbryzak@redhat.com",
		},
	}
	states.SetVerificationRequired(userSignup, true)

	err := s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err = s.Application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.Username, "+1NUMBER")
	require.NoError(s.T(), err)

	userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(s.T(), err)
	reqValue := buf.String()

	params, err := url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
		userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+1NUMBER", params.Get("To"))
	require.Equal(s.T(), "1", userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsWhenCountContainsInvalidValue() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHTTPClientFactoryOption()

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
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey:                 "testuser@redhat.com",
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey:       "abc",
				toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Format(verificationservice.TimestampLayout),
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "sbryzak@redhat.com",
		},
	}
	states.SetVerificationRequired(userSignup, true)

	err := s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err = s.Application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.Username, "+1NUMBER")
	require.EqualError(s.T(), err, "daily limit exceeded: cannot generate new verification code")
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsDailyCounterExceeded() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHTTPClientFactoryOption()

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
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey:                 "testuser@redhat.com",
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey:       strconv.Itoa(cfg.Verification().DailyLimit()),
				toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Format(verificationservice.TimestampLayout),
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "sbryzak@redhat.com",
		},
	}
	states.SetVerificationRequired(userSignup, true)

	err := s.FakeUserSignupClient.Tracker.Add(userSignup)
	require.NoError(s.T(), err)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err = s.Application.VerificationService().InitVerification(ctx, userSignup.Name, userSignup.Spec.Username, "+1NUMBER")
	require.EqualError(s.T(), err, "daily limit exceeded: cannot generate new verification code", err.Error())
	require.Empty(s.T(), userSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsWhenPhoneNumberInUse() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHTTPClientFactoryOption()

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
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "alpha@foxtrot.com",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: phoneHash,
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "alpha@foxtrot.com",
		},
	}
	states.SetApprovedManually(alphaUserSignup, true)

	err := s.FakeUserSignupClient.Tracker.Add(alphaUserSignup)
	require.NoError(s.T(), err)

	bravoUserSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bravo",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "bravo@foxtrot.com",
			},
			Labels: map[string]string{},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "bravo@foxtrot.com",
		},
	}
	states.SetVerificationRequired(bravoUserSignup, true)

	err = s.FakeUserSignupClient.Tracker.Add(bravoUserSignup)
	require.NoError(s.T(), err)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err = s.Application.VerificationService().InitVerification(ctx, bravoUserSignup.Name, bravoUserSignup.Spec.Username, e164PhoneNumber)
	require.Error(s.T(), err)
	require.Equal(s.T(), "phone number already in use: cannot register using phone number: +19875551122", err.Error())

	// Reload bravoUserSignup
	bravoUserSignup, err = s.FakeUserSignupClient.Get(bravoUserSignup.Name)
	require.NoError(s.T(), err)

	require.Empty(s.T(), bravoUserSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationOKWhenPhoneNumberInUseByDeactivatedUserSignup() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHTTPClientFactoryOption()

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
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "alpha@foxtrot.com",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: phoneHash,
				toolchainv1alpha1.UserSignupStateLabelKey:         toolchainv1alpha1.UserSignupStateLabelValueDeactivated,
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "alpha@foxtrot.com",
		},
	}
	states.SetApprovedManually(alphaUserSignup, true)
	states.SetDeactivated(alphaUserSignup, true)

	err := s.FakeUserSignupClient.Tracker.Add(alphaUserSignup)
	require.NoError(s.T(), err)

	bravoUserSignup := &toolchainv1alpha1.UserSignup{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bravo",
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey: "bravo@foxtrot.com",
			},
			Labels: map[string]string{},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			Username: "bravo@foxtrot.com",
		},
	}
	states.SetVerificationRequired(bravoUserSignup, true)

	err = s.FakeUserSignupClient.Tracker.Add(bravoUserSignup)
	require.NoError(s.T(), err)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err = s.Application.VerificationService().InitVerification(ctx, bravoUserSignup.Name, bravoUserSignup.Spec.Username, e164PhoneNumber)
	require.NoError(s.T(), err)

	// Reload bravoUserSignup
	bravoUserSignup, err = s.FakeUserSignupClient.Get(bravoUserSignup.Name)
	require.NoError(s.T(), err)

	// Just confirm that verification has been initialized by testing whether a verification code has been set
	require.NotEmpty(s.T(), bravoUserSignup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey])
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
					toolchainv1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}
		states.SetVerificationRequired(userSignup, true)

		err := s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.Username, "123456")
		require.NoError(s.T(), err)

		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		require.False(s.T(), states.VerificationRequired(userSignup))
	})

	s.T().Run("verification ok for usersignup with username identifier", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "employee085",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserSignupUserEmailAnnotationKey:        "employee085@redhat.com",
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "654321",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				Username: "employee085@redhat.com",
			},
		}
		states.SetVerificationRequired(userSignup, true)

		err := s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().VerifyPhoneCode(ctx, "", "employee085", "654321")
		require.NoError(s.T(), err)

		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		require.False(s.T(), states.VerificationRequired(userSignup))
	})

	s.T().Run("when verification code is invalid", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "000000",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		err := s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		require.NoError(s.T(), err)

		err = s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.Username, "123456")
		require.Error(s.T(), err)
		e := &crterrors.Error{}
		require.True(s.T(), errors.As(err, &e))
		require.Equal(s.T(), "invalid code: the provided code is invalid", e.Error())
		require.Equal(s.T(), http.StatusForbidden, int(e.Code))
	})

	s.T().Run("when verification code has expired", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(-10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		err := s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		require.NoError(s.T(), err)
		err = s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.Username, "123456")
		e := &crterrors.Error{}
		require.True(s.T(), errors.As(err, &e))
		require.Equal(s.T(), "expired: verification code expired", e.Error())
		require.Equal(s.T(), http.StatusForbidden, int(e.Code))
	})

	s.T().Run("when verifications exceeded maximum attempts", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "3",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		err := s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		require.NoError(s.T(), err)
		err = s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.Username, "123456")
		require.EqualError(s.T(), err, "too many verification attempts", err.Error())
	})

	s.T().Run("when verifications attempts has invalid value", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "ABC",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		err := s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		require.NoError(s.T(), err)
		err = s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.Username, "123456")
		require.EqualError(s.T(), err, "too many verification attempts", err.Error())

		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "3", userSignup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey])
	})

	s.T().Run("when verifications expiry is corrupt", func(t *testing.T) {

		userSignup := &toolchainv1alpha1.UserSignup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "123",
				Namespace: configuration.Namespace(),
				Annotations: map[string]string{
					toolchainv1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					toolchainv1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					toolchainv1alpha1.UserVerificationExpiryAnnotationKey:     "ABC",
				},
				Labels: map[string]string{
					toolchainv1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: toolchainv1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		err := s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		require.NoError(s.T(), err)
		err = s.FakeUserSignupClient.Tracker.Add(userSignup)
		require.NoError(s.T(), err)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err = s.Application.VerificationService().VerifyPhoneCode(ctx, userSignup.Name, userSignup.Spec.Username, "123456")
		require.EqualError(s.T(), err, "parsing time \"ABC\" as \"2006-01-02T15:04:05.000Z07:00\": cannot parse \"ABC\" as \"2006\": error parsing expiry timestamp", err.Error())
	})
}

func (s *TestVerificationServiceSuite) TestVerifyActivationCode() {

	// given

	cfg := configuration.GetRegistrationServiceConfig()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	s.T().Run("verification ok", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second)) // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event")
		err := s.setupFakeClients(userSignup, event)
		require.NoError(t, err)

		// when
		err = s.Application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.Username, event.Name)

		// then
		require.NoError(t, err)
		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(t, err)
		require.False(t, states.VerificationRequired(userSignup))
	})

	s.T().Run("last user to signup", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second))                        // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithActivationCount(9)) // one seat left
		err := s.setupFakeClients(userSignup, event)
		require.NoError(t, err)

		// when
		err = s.Application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.Username, event.Name)

		// then
		require.NoError(t, err)
		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(t, err)
		require.False(t, states.VerificationRequired(userSignup))
	})

	s.T().Run("when too many attempts made", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(
			testusersignup.VerificationRequired(time.Second), // just signed up
			testusersignup.WithVerificationAttempts(cfg.Verification().AttemptsAllowed()))
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event")
		err := s.setupFakeClients(userSignup, event)
		require.NoError(t, err)

		// when
		err = s.Application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.Username, event.Name)

		// then
		require.EqualError(t, err, "too many verification attempts: 3")
		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(t, err)
		require.True(t, states.VerificationRequired(userSignup)) // unchanged
	})

	s.T().Run("when invalid code", func(t *testing.T) {

		t.Run("first attempt", func(t *testing.T) {
			// given
			userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second)) // just signed up
			err := s.setupFakeClients(userSignup)
			require.NoError(t, err)

			// when
			err = s.Application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.Username, "invalid")

			// then
			require.EqualError(t, err, "invalid code: the provided code is invalid")
			userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
			require.NoError(t, err)
			require.True(t, states.VerificationRequired(userSignup))                                              // unchanged
			assert.Equal(t, "1", userSignup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		})

		t.Run("second attempt", func(t *testing.T) {
			// given
			userSignup := testusersignup.NewUserSignup(
				testusersignup.VerificationRequired(time.Second), // just signed up
				testusersignup.WithVerificationAttempts(2))       // already tried twice before
			err := s.setupFakeClients(userSignup)
			require.NoError(t, err)

			// when
			err = s.Application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.Username, "invalid")

			// then
			require.EqualError(t, err, "invalid code: the provided code is invalid")
			userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
			require.NoError(t, err)
			require.True(t, states.VerificationRequired(userSignup))                                              // unchanged
			assert.Equal(t, "3", userSignup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
		})
	})

	s.T().Run("when max attendees reached", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second))                         // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithActivationCount(10)) // same as default `spec.MaxAttendees`
		err := s.setupFakeClients(userSignup, event)
		require.NoError(t, err)

		// when
		err = s.Application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.Username, event.Name)

		// then
		require.EqualError(t, err, "invalid code: the event is full")
		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(t, err)
		require.True(t, states.VerificationRequired(userSignup))
		assert.Equal(t, "1", userSignup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
	})

	s.T().Run("when event not open yet", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second))                                          // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithStartTime(time.Now().Add(time.Hour))) // starting in 1hr
		err := s.setupFakeClients(userSignup, event)
		require.NoError(t, err)

		// when
		err = s.Application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.Username, event.Name)

		// then
		require.EqualError(t, err, "invalid code: the provided code is invalid")
		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(t, err)
		require.True(t, states.VerificationRequired(userSignup))
		assert.Equal(t, "1", userSignup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
	})

	s.T().Run("when event already closed", func(t *testing.T) {
		// given
		userSignup := testusersignup.NewUserSignup(testusersignup.VerificationRequired(time.Second))                                         // just signed up
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event", testsocialevent.WithEndTime(time.Now().Add(-time.Hour))) // ended 1hr ago
		err := s.setupFakeClients(userSignup, event)
		require.NoError(t, err)

		// when
		err = s.Application.VerificationService().VerifyActivationCode(ctx, userSignup.Name, userSignup.Spec.Username, event.Name)

		// then
		require.EqualError(t, err, "invalid code: the provided code is invalid")
		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(t, err)
		require.True(t, states.VerificationRequired(userSignup))
		assert.Equal(t, "1", userSignup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]) // incremented
	})
}

func (s *TestVerificationServiceSuite) setupFakeClients(objects ...runtime.Object) error {
	clientScheme := runtime.NewScheme()
	if err := toolchainv1alpha1.SchemeBuilder.AddToScheme(clientScheme); err != nil {
		return err
	}
	s.FakeUserSignupClient.Tracker = kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	s.FakeSocialEventClient.Tracker = kubetesting.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *toolchainv1alpha1.UserSignup:
			if err := s.FakeUserSignupClient.Tracker.Add(obj); err != nil {
				return err
			}
		case *toolchainv1alpha1.SocialEvent:
			if err := s.FakeSocialEventClient.Tracker.Add(obj); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected type of object: %T", obj)
		}
	}
	return nil
}
