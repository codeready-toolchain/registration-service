package verification

import (
	"crypto/rand"
	"fmt"

	"github.com/gin-gonic/gin"
	errs "k8s.io/apimachinery/pkg/api/errors"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/kevinburke/twilio-go"
)

const (
	codeCharset = "0123456789"
	codeLength  = 6
)

// ServiceConfiguration represents the config used for the verification service.
type ServiceConfiguration interface {
	GetTwilioAccountSID() string
	GetTwilioAuthToken() string
	GetTwilioFromNumber() string
	GetVerificationMessageTemplate() string
}

// Service represents the verification service for controllers.
type Service interface {
	SendVerification(ctx *gin.Context, signup *v1alpha1.UserSignup) error
}

// ServiceImpl represents the implementation of the verification service.
type ServiceImpl struct {
	config ServiceConfiguration
}

// NewVerificationService creates a service object for performing user verification
func NewVerificationService(cfg ServiceConfiguration) Service {
	s := &ServiceImpl{
		config: cfg,
	}
	return s
}

// SendVerification sends a verification message to the specified user
func (s *ServiceImpl) SendVerification(ctx *gin.Context, signup *v1alpha1.UserSignup) error {
	verificationCode, err := generateVerificationCode()
	if err != nil {
		return err
	}

	signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey] = verificationCode

	content := fmt.Sprintf(s.config.GetVerificationMessageTemplate(), verificationCode)

	toNumber := signup.Labels[v1alpha1.UserSignupPhoneNumberLabelKey]
	client := twilio.NewClient(s.config.GetTwilioAccountSID(), s.config.GetTwilioAuthToken(), nil)
	msg, err := client.Messages.SendMessage(s.config.GetTwilioFromNumber(), toNumber,
		content, nil)
	if err != nil {
		log.Error(ctx, err, fmt.Sprintf("error while sending, code: %d message: %s", msg.ErrorCode, msg.ErrorMessage))
		return errs.NewInternalError(err)
	}

	return nil
}

func generateVerificationCode() (string, error) {
	buf := make([]byte, codeLength)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	charsetLen := len(codeCharset)
	for i := 0; i < codeLength; i++ {
		buf[i] = codeCharset[int(buf[i])%charsetLen]
	}

	return string(buf), nil
}
