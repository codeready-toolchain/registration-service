package service_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"

	verificationservice "github.com/codeready-toolchain/registration-service/pkg/verification/service"

	"github.com/gin-gonic/gin"

	"gopkg.in/h2non/gock.v1"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codeready-toolchain/registration-service/test"
	test2 "github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/stretchr/testify/suite"
)

type TestVerificationServiceSuite struct {
	test.UnitTestSuite
	httpClient *http.Client
}

func TestRunVerificationServiceSuite(t *testing.T) {
	suite.Run(t, &TestVerificationServiceSuite{test.UnitTestSuite{}, nil})
}

func (s *TestVerificationServiceSuite) ServiceConfiguration(accountSID, authToken, fromNumber string) configuration.Configuration {
	restore := test2.SetEnvVarAndRestore(s.T(), k8sutil.WatchNamespaceEnvVar, "toolchain-host-operator")
	defer restore()

	baseConfig, _ := configuration.LoadConfig(test2.NewFakeClient(s.T()))

	return &mockVerificationConfig{
		ViperConfig:     *baseConfig,
		accountSID:      accountSID,
		authToken:       authToken,
		fromNumber:      fromNumber,
		messageTemplate: configuration.DefaultVerificationMessageTemplate,
		attemptsAllowed: 3,
		dailyLimit:      3,
		codeExpiry:      5,
	}
}

type mockVerificationConfig struct {
	configuration.ViperConfig
	accountSID      string
	authToken       string
	fromNumber      string
	messageTemplate string
	attemptsAllowed int
	dailyLimit      int
	codeExpiry      int
}

func (c *mockVerificationConfig) GetTwilioAccountSID() string {
	return c.accountSID
}

func (c *mockVerificationConfig) GetTwilioAuthToken() string {
	return c.authToken
}

func (c *mockVerificationConfig) GetTwilioFromNumber() string {
	return c.fromNumber
}

func (c *mockVerificationConfig) GetVerificationMessageTemplate() string {
	return c.messageTemplate
}

func (c *mockVerificationConfig) GetVerificationAttemptsAllowed() int {
	return c.attemptsAllowed
}

func (c *mockVerificationConfig) GetVerificationDailyLimit() int {
	return c.dailyLimit
}

func (c *mockVerificationConfig) GetVerificationCodeExpiresInMin() int {
	return c.codeExpiry
}

func (s *TestVerificationServiceSuite) SetHttpClientFactoryOption() {

	s.httpClient = &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(s.httpClient)

	serviceOption := func(svc *verificationservice.ServiceImpl) {
		svc.HttpClient = s.httpClient
	}

	opt := func(serviceFactory *factory.ServiceFactory) {
		serviceFactory.WithVerificationServiceOption(serviceOption)
	}

	s.WithFactoryOption(opt)
}

func (s *TestVerificationServiceSuite) TestInitVerification() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHttpClientFactoryOption()
	s.OverrideConfig(s.ServiceConfiguration("xxx", "yyy", "CodeReady"))

	defer gock.Off()

	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")

	var reqBody io.ReadCloser
	obs := func(request *http.Request, mock gock.Mock) {
		reqBody = request.Body
	}

	gock.Observe(obs)

	userSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "123",
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "sbryzak@redhat.com",
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "sbryzak@redhat.com",
			VerificationRequired: true,
		},
	}

	s.FakeUserSignupClient.Tracker.Add(userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := s.Application.VerificationService().InitVerification(ctx, userSignup.Name, "+1NUMBER")
	require.NoError(s.T(), err)

	userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(s.T(), err)
	reqValue := buf.String()

	params, err := url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
		userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+1NUMBER", params.Get("To"))
}

func (s *TestVerificationServiceSuite) TestInitVerificationPassesWhenMaxCountReachedAndTimestampElapsed() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHttpClientFactoryOption()
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	s.OverrideConfig(s.ServiceConfiguration("xxx", "yyy", "CodeReady"))

	now := time.Now()

	var reqBody io.ReadCloser
	obs := func(request *http.Request, mock gock.Mock) {
		reqBody = request.Body
	}

	gock.Observe(obs)

	userSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "123",
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey:                 "testuser@redhat.com",
				v1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Add(-25 * time.Hour).Format(verificationservice.TimestampLayout),
				v1alpha1.UserVerificationAttemptsAnnotationKey:            "3",
				v1alpha1.UserSignupVerificationCodeAnnotationKey:          "123456",
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "sbryzak@redhat.com",
			VerificationRequired: true,
		},
	}

	s.FakeUserSignupClient.Tracker.Add(userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := s.Application.VerificationService().InitVerification(ctx, userSignup.Name, "+1NUMBER")
	require.NoError(s.T(), err)

	userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(s.T(), err)
	reqValue := buf.String()

	params, err := url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Developer Sandbox for Red Hat OpenShift: Your verification code is %s",
		userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+1NUMBER", params.Get("To"))
	require.Equal(s.T(), "1", userSignup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsWhenCountContainsInvalidValue() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHttpClientFactoryOption()

	defer gock.Off()
	s.OverrideConfig(s.DefaultConfig())

	now := time.Now()

	userSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "123",
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey:                 "testuser@redhat.com",
				v1alpha1.UserSignupVerificationCounterAnnotationKey:       "abc",
				v1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Format(verificationservice.TimestampLayout),
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "sbryzak@redhat.com",
			VerificationRequired: true,
		},
	}

	s.FakeUserSignupClient.Tracker.Add(userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := s.Application.VerificationService().InitVerification(ctx, userSignup.Name, "+1NUMBER")
	require.Error(s.T(), err)
	require.Equal(s.T(), "daily limit exceeded:cannot generate new verification code", err.Error())
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsDailyCounterExceeded() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHttpClientFactoryOption()

	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	s.OverrideConfig(s.DefaultConfig())

	now := time.Now()

	userSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "123",
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey:                 "testuser@redhat.com",
				v1alpha1.UserSignupVerificationCounterAnnotationKey:       strconv.Itoa(s.Config().GetVerificationDailyLimit()),
				v1alpha1.UserSignupVerificationInitTimestampAnnotationKey: now.Format(verificationservice.TimestampLayout),
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "sbryzak@redhat.com",
			VerificationRequired: true,
		},
	}

	s.FakeUserSignupClient.Tracker.Add(userSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := s.Application.VerificationService().InitVerification(ctx, userSignup.Name, "+1NUMBER")
	require.Error(s.T(), err)
	require.Equal(s.T(), "daily limit exceeded:cannot generate new verification code", err.Error())

	require.Empty(s.T(), userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationFailsWhenPhoneNumberInUse() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHttpClientFactoryOption()

	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	s.OverrideConfig(s.DefaultConfig())

	e164PhoneNumber := "+19875551122"

	// calculate the phone number hash
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(e164PhoneNumber))
	phoneHash := hex.EncodeToString(md5hash.Sum(nil))

	alphaUserSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "alpha",
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "alpha@foxtrot.com",
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserPhoneHashLabelKey: phoneHash,
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "alpha@foxtrot.com",
			VerificationRequired: false,
			Approved:             true,
		},
	}

	s.FakeUserSignupClient.Tracker.Add(alphaUserSignup)

	bravoUserSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "bravo",
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "bravo@foxtrot.com",
			},
			Labels: map[string]string{},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "bravo@foxtrot.com",
			VerificationRequired: true,
		},
	}

	s.FakeUserSignupClient.Tracker.Add(bravoUserSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := s.Application.VerificationService().InitVerification(ctx, bravoUserSignup.Name, e164PhoneNumber)
	require.Error(s.T(), err)
	require.Equal(s.T(), "phone number already in use:cannot register using phone number: +19875551122", err.Error())

	// Reload bravoUserSignup
	bravoUserSignup, err = s.FakeUserSignupClient.Get(bravoUserSignup.Name)
	require.NoError(s.T(), err)

	require.Empty(s.T(), bravoUserSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestInitVerificationOKWhenPhoneNumberInUseByDeactivatedUserSignup() {
	// Setup gock to intercept calls made to the Twilio API
	s.SetHttpClientFactoryOption()

	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	defer gock.Off()
	s.OverrideConfig(s.DefaultConfig())

	e164PhoneNumber := "+19875553344"

	// calculate the phone number hash
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(e164PhoneNumber))
	phoneHash := hex.EncodeToString(md5hash.Sum(nil))

	alphaUserSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "alpha",
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "alpha@foxtrot.com",
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserPhoneHashLabelKey: phoneHash,
				v1alpha1.UserSignupStateLabelKey:         v1alpha1.UserSignupStateLabelValueDeactivated,
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "alpha@foxtrot.com",
			VerificationRequired: false,
			Deactivated:          true,
			Approved:             true,
		},
	}

	s.FakeUserSignupClient.Tracker.Add(alphaUserSignup)

	bravoUserSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "bravo",
			Namespace: s.Config().GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "bravo@foxtrot.com",
			},
			Labels: map[string]string{},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username:             "bravo@foxtrot.com",
			VerificationRequired: true,
		},
	}

	s.FakeUserSignupClient.Tracker.Add(bravoUserSignup)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := s.Application.VerificationService().InitVerification(ctx, bravoUserSignup.Name, e164PhoneNumber)
	require.NoError(s.T(), err)

	// Reload bravoUserSignup
	bravoUserSignup, err = s.FakeUserSignupClient.Get(bravoUserSignup.Name)
	require.NoError(s.T(), err)

	// Just confirm that verification has been initialized by testing whether a verification code has been set
	require.NotEmpty(s.T(), bravoUserSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestVerifyCode() {
	// given
	now := time.Now()

	s.T().Run("verification ok", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: s.Config().GetNamespace(),
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username:             "sbryzak@redhat.com",
				VerificationRequired: true,
			},
		}

		s.FakeUserSignupClient.Tracker.Add(userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := s.Application.VerificationService().VerifyCode(ctx, userSignup.Name, "123456")
		require.NoError(s.T(), err)

		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		require.False(s.T(), userSignup.Spec.VerificationRequired)
	})

	s.T().Run("when verification code is invalid", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: s.Config().GetNamespace(),
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "000000",
					v1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		s.FakeUserSignupClient.Tracker.Add(userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := s.Application.VerificationService().VerifyCode(ctx, userSignup.Name, "123456")
		require.Error(s.T(), err)
		require.IsType(s.T(), err, &errors.Error{})
		require.Equal(s.T(), "invalid code:the provided code is invalid", err.(*errors.Error).Error())
		require.Equal(s.T(), http.StatusForbidden, int(err.(*errors.Error).Code))
	})

	s.T().Run("when verification code has expired", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: s.Config().GetNamespace(),
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(-10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		s.FakeUserSignupClient.Tracker.Add(userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := s.Application.VerificationService().VerifyCode(ctx, userSignup.Name, "123456")
		require.Error(s.T(), err)
		require.IsType(s.T(), err, &errors.Error{})
		require.Equal(s.T(), "expired:verification code expired", err.(*errors.Error).Error())
		require.Equal(s.T(), http.StatusForbidden, int(err.(*errors.Error).Code))
	})

	s.T().Run("when verifications exceeded maximum attempts", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: s.Config().GetNamespace(),
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "3",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		s.FakeUserSignupClient.Tracker.Add(userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := s.Application.VerificationService().VerifyCode(ctx, userSignup.Name, "123456")
		require.Error(s.T(), err)
		require.Equal(s.T(), "too many verification attempts:", err.Error())
	})

	s.T().Run("when verifications attempts has invalid value", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: s.Config().GetNamespace(),
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "ABC",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verificationservice.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		s.FakeUserSignupClient.Tracker.Add(userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := s.Application.VerificationService().VerifyCode(ctx, userSignup.Name, "123456")
		require.Error(s.T(), err)
		require.Equal(s.T(), "too many verification attempts:", err.Error())

		userSignup, err = s.FakeUserSignupClient.Get(userSignup.Name)
		require.NoError(s.T(), err)

		require.Equal(s.T(), "3", userSignup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey])
	})

	s.T().Run("when verifications expiry is corrupt", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: s.Config().GetNamespace(),
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:     "ABC",
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		s.FakeUserSignupClient.Delete(userSignup.Name, nil)
		s.FakeUserSignupClient.Tracker.Add(userSignup)

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		err := s.Application.VerificationService().VerifyCode(ctx, userSignup.Name, "123456")
		require.Error(s.T(), err)
		require.Equal(s.T(), "parsing time \"ABC\" as \"2006-01-02T15:04:05.000Z07:00\": cannot parse \"ABC\" as \"2006\":error parsing expiry timestamp", err.Error())
	})

}
