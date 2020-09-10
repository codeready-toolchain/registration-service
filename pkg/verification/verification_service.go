package verification

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/errors"

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
	GetVerificationDailyLimit() int
}

// Service represents the verification service for controllers.
type Service interface {
	InitVerification(ctx *gin.Context, signup *v1alpha1.UserSignup, countryCode, phoneNumber string) (*v1alpha1.UserSignup, error)
	VerifyCode(signup *v1alpha1.UserSignup, code string) (*v1alpha1.UserSignup, error)
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

// InitVerification sends a verification message to the specified user.  If successful,
// the user will receive a verification SMS.
func (s *ServiceImpl) InitVerification(ctx *gin.Context, signup *v1alpha1.UserSignup, countryCode, phoneNumber string) (*v1alpha1.UserSignup, error) {
	// get phone number hash
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(countryCode + phoneNumber))
	phoneHash := hex.EncodeToString(md5hash.Sum(nil))

	signup.Labels[v1alpha1.UserSignupUserPhoneHashLabelKey] = phoneHash

	now := time.Now()

	// If 24 hours has passed since the verification timestamp, then reset the timestamp and verification attempts
	ts, err := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserSignupVerificationInitTimestampAnnotationKey])
	if err != nil || (err == nil && now.After(ts.Add(24*time.Hour))) {
		// Set a new timestamp
		signup.Annotations[v1alpha1.UserSignupVerificationInitTimestampAnnotationKey] = now.Format(TimestampLayout)
		signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey] = "0"
	}

	// get the annotation counter
	annotationCounter := signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey]
	var counter int
	dailyLimit := s.config.GetVerificationDailyLimit()
	if annotationCounter == "" {
		counter = 0
	} else {
		counter, err = strconv.Atoi(annotationCounter)
		if err != nil {
			// We shouldn't get an error here, but if we do, we should probably set verification attempts to daily limit
			// so that we at least now have a valid value
			signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(dailyLimit)
			return signup, errors.NewInternalError(err, fmt.Sprintf("error when retrieving counter annotation for UserSignup %s, set to daily limit", signup.GetName()))
		}
	}

	// check if counter has exceeded the limit of daily limit - if at limit error out
	if counter >= dailyLimit {
		err := errors.NewForbiddenError("daily limit exceeded", "cannot generate new verification code")
		log.Error(ctx, err, fmt.Sprintf("%d attempts made. the daily limit of %d has been exceeded", counter, dailyLimit))
		return signup, err
	}

	// generate verification code
	code, err := generateVerificationCode()
	if err != nil {
		log.Errorf(ctx, nil, "verification code could not be generated")
		return signup, ctx.AbortWithError(http.StatusInternalServerError, err)
	}

	// set the usersignup annotations
	signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(counter + 1)
	signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey] = code

	content := fmt.Sprintf(s.config.GetVerificationMessageTemplate(), code)
	toNumber := countryCode + phoneNumber
	client := twilio.NewClient(s.config.GetTwilioAccountSID(), s.config.GetTwilioAuthToken(), s.HttpClient)
	msg, err := client.Messages.SendMessage(s.config.GetTwilioFromNumber(), toNumber,
		content, nil)
	if err != nil {
		log.Error(ctx, err, fmt.Sprintf("error while sending, code: %d message: %s", msg.ErrorCode, msg.ErrorMessage))
		return signup, err
	}

	return signup, nil
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

// VerifyCode validates the user's phone verification code.  It updates the specified UserSignup value, so even
// if an error is returned by this function the caller should still process changes to it
func (s *ServiceImpl) VerifyCode(signup *v1alpha1.UserSignup, code string) (*v1alpha1.UserSignup, error) {
	now := time.Now()

	// If 24 hours has passed since the verification timestamp, then reset the timestamp and verification attempts
	ts, err := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserSignupVerificationTimestampAnnotationKey])
	if err != nil || (err == nil && now.After(ts.Add(24*time.Hour))) {
		// Set a new timestamp
		signup.Annotations[v1alpha1.UserSignupVerificationTimestampAnnotationKey] = now.Format(TimestampLayout)
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = "0"
	}

	attemptsMade, err := strconv.Atoi(signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey])
	if err != nil {
		// We shouldn't get an error here, but if we do, we should probably set verification attempts to max allowed
		// so that we at least now have a valid value
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(s.config.GetVerificationAttemptsAllowed())
		return signup, err
	}

	// If the user has made more attempts than is allowed per day, return an error
	if attemptsMade >= s.config.GetVerificationAttemptsAllowed() {
		return signup, errors.NewTooManyRequestsError("too many verification attempts", "")
	}

	exp, err := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserVerificationExpiryAnnotationKey])
	if err != nil {
		// If the verification expiry timestamp is corrupt or missing, then return an error
		return signup, errors.NewInternalError(err, "error parsing expiry timestamp")
	}

	if now.After(exp) {
		// If it is now past the expiry timestamp for the verification code, return a 403 Forbidden error
		return signup, errors.NewForbiddenError("expired", "verification code expired")
	}

	if code != signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey] {
		// The code doesn't match
		attemptsMade++
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
		return signup, errors.NewForbiddenError("invalid code", "the provided code is invalid")
	}

	// If the code matches then set VerificationRequired to false, reset other verification annotations
	signup.Spec.VerificationRequired = false
	delete(signup.Annotations, v1alpha1.UserSignupVerificationCodeAnnotationKey)
	delete(signup.Annotations, v1alpha1.UserVerificationAttemptsAnnotationKey)
	delete(signup.Annotations, v1alpha1.UserSignupVerificationCounterAnnotationKey)
	delete(signup.Annotations, v1alpha1.UserSignupVerificationTimestampAnnotationKey)
	delete(signup.Annotations, v1alpha1.UserSignupVerificationInitTimestampAnnotationKey)
	delete(signup.Annotations, v1alpha1.UserVerificationExpiryAnnotationKey)

	return signup, nil
}
