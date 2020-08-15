package verification_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gopkg.in/h2non/gock.v1"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"

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

func NewMockVerificationConfig(accountSID, authToken, fromNumber string) verification.ServiceConfiguration {
	return &mockVerificationConfig{
		accountSID:      accountSID,
		authToken:       authToken,
		fromNumber:      fromNumber,
		messageTemplate: configuration.DefaultVerificationMessageTemplate,
	}
}

func (s *TestVerificationServiceSuite) TestVerify() {
	defer gock.Off()

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
				v1alpha1.UserSignupPhoneNumberLabelKey: "+1NUMBER",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username: "sbryzak@redhat.com",
		},
	}

	err := svc.SendVerification(ctx, userSignup)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])
}

func (s *TestVerificationServiceSuite) createVerificationService() (verification.Service, *http.Client) {
	cfg := NewMockVerificationConfig(
		"xxx",
		"yyy",
		"CodeReady",
	)

	httpClient := &http.Client{Transport: &http.Transport{}}

	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")
	gock.InterceptClient(httpClient)

	var mockClientOpt verification.VerificationServiceOption
	mockClientOpt = func(svc *verification.ServiceImpl) {
		svc.HttpClient = httpClient
	}

	svc := verification.NewVerificationService(cfg, mockClientOpt)

	return svc, httpClient
}
