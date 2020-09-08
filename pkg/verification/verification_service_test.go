package verification_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"gopkg.in/h2non/gock.v1"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codeready-toolchain/registration-service/pkg/verification"

	"github.com/codeready-toolchain/registration-service/test"
	"github.com/stretchr/testify/suite"
)

type TestVerificationServiceSuite struct {
	test.UnitTestSuite
}

func TestRunVerificationServiceSuite(t *testing.T) {
	suite.Run(t, &TestVerificationServiceSuite{test.UnitTestSuite{}})
}

type mockVerificationConfig struct {
	accountSID      string
	authToken       string
	fromNumber      string
	messageTemplate string
	attemptsAllowed int
	dailyLimit      int
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

func NewMockVerificationConfig(accountSID, authToken, fromNumber string) verification.ServiceConfiguration {
	return &mockVerificationConfig{
		accountSID:      accountSID,
		authToken:       authToken,
		fromNumber:      fromNumber,
		messageTemplate: configuration.DefaultVerificationMessageTemplate,
		attemptsAllowed: 3,
	}
}

func (s *TestVerificationServiceSuite) TestSendVerification() {
	defer gock.Off()
	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	svc, _ := s.createVerificationService()

	var reqBody io.ReadCloser
	obs := func(request *http.Request, mock gock.Mock) {
		reqBody = request.Body
	}

	gock.Observe(obs)

	userSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "123",
			Namespace: "test",
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
				v1alpha1.UserSignupVerificationCodeAnnotationKey: "1234",
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username: "sbryzak@redhat.com",
		},
	}

	_, err := svc.InitVerification(ctx, userSignup, "+1", "NUMBER")
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])

	buf := new(bytes.Buffer)
	buf.ReadFrom(reqBody)
	reqValue := buf.String()

	params, err := url.ParseQuery(reqValue)
	require.NoError(s.T(), err)
	require.Equal(s.T(), fmt.Sprintf("Your verification code for Red Hat Developer Sandbox is: %s",
		userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey]),
		params.Get("Body"))
	require.Equal(s.T(), "CodeReady", params.Get("From"))
	require.Equal(s.T(), "+1NUMBER", params.Get("To"))
}

func (s *TestVerificationServiceSuite) TestSendVerifyMessageFails() {
	defer gock.Off()
	gock.New("https://api.twilio.com").
		Reply(http.StatusInternalServerError).
		BodyString("")

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	svc, _ := s.createVerificationService()

	userSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "123",
			Namespace: "test",
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "sbryzak@redhat.com",
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username: "sbryzak@redhat.com",
		},
	}

	_, err := svc.InitVerification(ctx, userSignup, "+1", "NUMBER")
	require.Error(s.T(), err)
	require.Equal(s.T(), "invalid response body: ", err.Error())

	require.NotEmpty(s.T(), userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) TestVerifyCode() {
	// given
	svc, _ := s.createVerificationService()
	now := time.Now()

	s.T().Run("verification ok", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: "test",
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verification.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		_, err := svc.VerifyCode(userSignup, "123456")
		require.NoError(s.T(), err)
		require.False(s.T(), userSignup.Spec.VerificationRequired)
	})

	s.T().Run("when verification code is invalid", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: "test",
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "000000",
					v1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(10 * time.Second).Format(verification.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		_, err := svc.VerifyCode(userSignup, "123456")
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
				Namespace: "test",
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:        "sbryzak@redhat.com",
					v1alpha1.UserVerificationAttemptsAnnotationKey:   "0",
					v1alpha1.UserSignupVerificationCodeAnnotationKey: "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:     now.Add(-10 * time.Second).Format(verification.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		_, err := svc.VerifyCode(userSignup, "123456")
		require.Error(s.T(), err)
		require.IsType(s.T(), err, &errors.Error{})
		require.Equal(s.T(), "expired:verification code expired", err.(*errors.Error).Error())
		require.Equal(s.T(), http.StatusForbidden, int(err.(*errors.Error).Code))
	})

	s.T().Run("when previous verifications exceeded maximum attempts but timestamp has elapsed", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: "test",
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:             "sbryzak@redhat.com",
					v1alpha1.UserSignupVerificationTimestampAnnotationKey: now.Add(-25 * time.Hour).Format(verification.TimestampLayout),
					v1alpha1.UserVerificationAttemptsAnnotationKey:        "3",
					v1alpha1.UserSignupVerificationCodeAnnotationKey:      "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:          now.Add(10 * time.Second).Format(verification.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		_, err := svc.VerifyCode(userSignup, "123456")
		require.NoError(s.T(), err)
	})

	s.T().Run("when verifications exceeded maximum attempts", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: "test",
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:             "sbryzak@redhat.com",
					v1alpha1.UserSignupVerificationTimestampAnnotationKey: now.Add(-1 * time.Minute).Format(verification.TimestampLayout),
					v1alpha1.UserVerificationAttemptsAnnotationKey:        "3",
					v1alpha1.UserSignupVerificationCodeAnnotationKey:      "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:          now.Add(10 * time.Second).Format(verification.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		_, err := svc.VerifyCode(userSignup, "123456")
		require.Error(s.T(), err)
		require.Equal(s.T(), "too many verification attempts:", err.Error())
	})

	s.T().Run("when verifications attempts has invalid value", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: "test",
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:             "sbryzak@redhat.com",
					v1alpha1.UserSignupVerificationTimestampAnnotationKey: now.Add(-1 * time.Minute).Format(verification.TimestampLayout),
					v1alpha1.UserVerificationAttemptsAnnotationKey:        "ABC",
					v1alpha1.UserSignupVerificationCodeAnnotationKey:      "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:          now.Add(10 * time.Second).Format(verification.TimestampLayout),
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		_, err := svc.VerifyCode(userSignup, "123456")
		require.Error(s.T(), err)
		require.Equal(s.T(), "strconv.Atoi: parsing \"ABC\": invalid syntax", err.Error())
		require.Equal(s.T(), "3", userSignup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey])
	})

	s.T().Run("when verifications expiry is corrupt", func(t *testing.T) {

		userSignup := &v1alpha1.UserSignup{
			TypeMeta: v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{
				Name:      "123",
				Namespace: "test",
				Annotations: map[string]string{
					v1alpha1.UserSignupUserEmailAnnotationKey:             "sbryzak@redhat.com",
					v1alpha1.UserSignupVerificationTimestampAnnotationKey: now.Add(-1 * time.Minute).Format(verification.TimestampLayout),
					v1alpha1.UserVerificationAttemptsAnnotationKey:        "0",
					v1alpha1.UserSignupVerificationCodeAnnotationKey:      "123456",
					v1alpha1.UserVerificationExpiryAnnotationKey:          "ABC",
				},
				Labels: map[string]string{
					v1alpha1.UserSignupUserPhoneHashLabelKey: "+1NUMBER",
				},
			},
			Spec: v1alpha1.UserSignupSpec{
				Username: "sbryzak@redhat.com",
			},
		}

		_, err := svc.VerifyCode(userSignup, "123456")
		require.Error(s.T(), err)
		require.Equal(s.T(), "parsing time \"ABC\" as \"2006-01-02T15:04:05.000Z07:00\": cannot parse \"ABC\" as \"2006\":error parsing expiry timestamp", err.Error())
	})
}

func (s *TestVerificationServiceSuite) createVerificationService() (verification.Service, *http.Client) {
	cfg := NewMockVerificationConfig(
		"xxx",
		"yyy",
		"CodeReady",
	)

	httpClient := &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(httpClient)

	var mockClientOpt verification.VerificationServiceOption
	mockClientOpt = func(svc *verification.ServiceImpl) {
		svc.HttpClient = httpClient
	}

	svc := verification.NewVerificationService(cfg, mockClientOpt)

	return svc, httpClient
}
