package verification_test

import (
	"fmt"
	"testing"

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
	accountSID string
	authToken  string
}

func (c *mockVerificationConfig) GetTwilioAccountSID() string {
	return c.accountSID
}

func (c *mockVerificationConfig) GetTwilioAuthToken() string {
	return c.authToken
}

func NewMockVerificationConfig(accountSID, authToken string) verification.ServiceConfiguration {
	return &mockVerificationConfig{
		accountSID: accountSID,
		authToken:  authToken,
	}
}

func (s *TestVerificationServiceSuite) TestVerify() {
	cfg := NewMockVerificationConfig(
		"AC2accde14acb44b6b83c943d90b408e30",
		"8fcb716273bd0decb51b1612437f2b99")

	svc, err := verification.NewVerificationService(cfg)
	require.NoError(s.T(), err)

	userSignup := &v1alpha1.UserSignup{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      "123",
			Namespace: "test",
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey: "sbryzak@redhat.com",
				v1alpha1.UserSignupPhoneNumberLabelKey:    "+61438725577",
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			Username: "john@gmail.com",
		},
	}

	err = svc.SendVerification(userSignup)
	require.NoError(s.T(), err)

	fmt.Printf("Verification code: %s\n", userSignup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey])
}
