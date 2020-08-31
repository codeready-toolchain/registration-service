package verification

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"time"

	errors3 "github.com/codeready-toolchain/registration-service/pkg/errors"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-gonic/gin"
	"github.com/kevinburke/twilio-go"
)

const (
	codeCharset = "0123456789"
	codeLength  = 6

	TimestampLayout = "2006-01-02T15:04:05.000Z07:00"
)

// ServiceConfiguration represents the config used for the verification service.
type ServiceConfiguration interface {
	GetTwilioAccountSID() string
	GetTwilioAuthToken() string
	GetTwilioFromNumber() string
	GetVerificationMessageTemplate() string
	GetVerificationAttemptsAllowed() int
}

// Service represents the verification service for controllers.
type Service interface {
	SendVerification(ctx *gin.Context, signup *v1alpha1.UserSignup) error
	GenerateVerificationCode() (string, error)
	VerifyCode(ctx *gin.Context, signup *v1alpha1.UserSignup, code string) error
}

// ServiceImpl represents the implementation of the verification service.
type ServiceImpl struct {
	config     ServiceConfiguration
	HttpClient *http.Client
}

type VerificationServiceOption func(svc *ServiceImpl)

// NewVerificationService creates a service object for performing user verification
func NewVerificationService(cfg ServiceConfiguration, opts ...VerificationServiceOption) Service {
	s := &ServiceImpl{
		config: cfg,
	}

	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SendVerification sends a verification message to the specified user.  If successful,
// the user will receive a verification SMS.
func (s *ServiceImpl) SendVerification(ctx *gin.Context, signup *v1alpha1.UserSignup) error {
	verificationCode := signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey]

	content := fmt.Sprintf(s.config.GetVerificationMessageTemplate(), verificationCode)

	toNumber := signup.Labels[v1alpha1.UserSignupPhoneNumberLabelKey]
	client := twilio.NewClient(s.config.GetTwilioAccountSID(), s.config.GetTwilioAuthToken(), s.HttpClient)
	msg, err := client.Messages.SendMessage(s.config.GetTwilioFromNumber(), toNumber,
		content, nil)
	if err != nil {
		log.Error(ctx, err, fmt.Sprintf("error while sending, code: %d message: %s", msg.ErrorCode, msg.ErrorMessage))
		return err
	}

	return nil
}

func (s *ServiceImpl) GenerateVerificationCode() (string, error) {
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

// VerifyCode validates the user's phone verification code.  It updates the specified UserSignup value, so even
// if an error is returned by this function the caller should still process changes to it
func (s *ServiceImpl) VerifyCode(ctx *gin.Context, signup *v1alpha1.UserSignup, code string) error {

	now := time.Now()

	// If 24 hours has passed since the verification timestamp, then reset the timestamp and verification attempts
	ts, err := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserSignupVerificationTimestampAnnotationKey])
	if err != nil || (err == nil && now.After(ts.Add(24*time.Hour))) {
		// Set a new timestamp
		signup.Annotations[v1alpha1.UserSignupVerificationTimestampAnnotationKey] = now.Format(TimestampLayout)
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(0)
	}

	attemptsMade, err := strconv.Atoi(signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey])
	if err != nil {
		// We shouldn't get an error here, but if we do, we should probably set verification attempts to max allowed
		// so that we at least now have a valid value
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(s.config.GetVerificationAttemptsAllowed())
		return err
	}

	// If the user has made more attempts than is allowed per day, return an error
	if attemptsMade >= s.config.GetVerificationAttemptsAllowed() {
		return errors3.NewTooManyRequestsError("too many verification attempts", "")
	}

	exp, err := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserVerificationExpiryAnnotationKey])
	if err != nil {
		// If the verification expiry timestamp is corrupt or missing, then return an error
		return errors3.NewInternalError(err, "error parsing expiry timestamp")
	}

	if now.After(exp) {
		// If it is now past the expiry timestamp for the verification code, return a 403 Forbidden error
		return errors3.NewForbiddenError("expired", "verification code expired")
	}

	// If the code matches then set VerificationRequired to false, reset other verification annotations
	if code == signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey] {
		signup.Spec.VerificationRequired = false
		delete(signup.Annotations, v1alpha1.UserSignupVerificationCodeAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserVerificationAttemptsAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserSignupVerificationCounterAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserSignupVerificationTimestampAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserVerificationExpiryAnnotationKey)
		return nil
	}

	// The code doesn't match
	attemptsMade++
	signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
	return errors3.NewForbiddenError("invalid code", "the provided code is invalid")
}
