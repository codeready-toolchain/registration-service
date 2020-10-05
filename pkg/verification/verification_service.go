package verification

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"

	"github.com/codeready-toolchain/registration-service/pkg/errors"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-gonic/gin"
	"github.com/kevinburke/twilio-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	GetVerificationCodeExpiresInMin() int
}

// ServiceImpl represents the implementation of the verification service.
type ServiceImpl struct {
	base.BaseService
	config     ServiceConfiguration
	HttpClient *http.Client
}

type VerificationServiceOption func(svc *ServiceImpl)

// NewVerificationService creates a service object for performing user verification
func NewVerificationService(context servicecontext.ServiceContext, cfg ServiceConfiguration, opts ...VerificationServiceOption) service.VerificationService {
	s := &ServiceImpl{
		BaseService: base.NewBaseService(context),
		config:      cfg,
	}

	for _, opt := range opts {
		opt(s)
	}
	return s
}

// InitVerification sends a verification message to the specified user.  If successful,
// the user will receive a verification SMS.
func (s *ServiceImpl) InitVerification(ctx *gin.Context, userID string, e164PhoneNumber string) error {
	signup, err := s.Services().SignupService().GetUserSignup(userID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(ctx, err, "usersignup not found")
			errors.AbortWithError(ctx, http.StatusNotFound, err, "usersignup not found")
			return
		}
		log.Error(ctx, err, "error retrieving usersignup")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, fmt.Sprintf("error retrieving usersignup: %s", userID))
		return
	}

	// check that verification is required before proceeding
	if signup.Spec.VerificationRequired == false {
		log.Errorf(ctx, errors.NewForbiddenError("forbidden request", "verification code will not be sent"), "phone verification not required for usersignup: %s", userID)
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	err = s.Services().SignupService().PhoneNumberAlreadyInUse(userID, e164PhoneNumber)
	if err != nil {
		if apierrors.IsForbidden(err) {
			log.Errorf(ctx, err, "phone number already in use, cannot register using phone number: %s", e164PhoneNumber)
			errors.AbortWithError(ctx, http.StatusForbidden, err, fmt.Sprintf("phone number already in use, cannot register using phone number: %s", e164PhoneNumber))
			return
		}
		log.Error(ctx, err, "error while looking up users by phone number")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "could not lookup users by phone number")
		return
	}

	// calculate the phone number hash
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(e164PhoneNumber))
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
			return errors.NewInternalError(err, fmt.Sprintf("error when retrieving counter annotation for UserSignup %s, set to daily limit", signup.GetName()))
		}
	}

	// check if counter has exceeded the limit of daily limit - if at limit error out
	if counter >= dailyLimit {
		err := errors.NewForbiddenError("daily limit exceeded", "cannot generate new verification code")
		log.Error(ctx, err, fmt.Sprintf("%d attempts made. the daily limit of %d has been exceeded", counter, dailyLimit))
		return err
	}

	// generate verification code
	code, err := generateVerificationCode()
	if err != nil {
		log.Errorf(ctx, err, "verification code could not be generated")
		return ctx.AbortWithError(http.StatusInternalServerError, err)
	}

	// set the usersignup annotations
	signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(counter + 1)
	signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey] = code
	signup.Annotations[v1alpha1.UserVerificationExpiryAnnotationKey] = now.Add(
		time.Duration(s.config.GetVerificationCodeExpiresInMin()) * time.Minute).Format(TimestampLayout)

	content := fmt.Sprintf(s.config.GetVerificationMessageTemplate(), code)
	client := twilio.NewClient(s.config.GetTwilioAccountSID(), s.config.GetTwilioAuthToken(), s.HttpClient)
	msg, err := client.Messages.SendMessage(s.config.GetTwilioFromNumber(), e164PhoneNumber, content, nil)
	if err != nil {
		log.Error(ctx, err, fmt.Sprintf("error while sending, code: %d message: %s", msg.ErrorCode, msg.ErrorMessage))
		return err
	}

	_, err2 := s.Services().SignupService().UpdateUserSignup(signup)
	if err2 != nil {
		log.Error(ctx, err2, "error while updating UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err2, "error while updating UserSignup resource")

		if err != nil {
			log.Error(ctx, err, "error initiating user verification")
		}

		return
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

// VerifyCode validates the user's phone verification code.  It updates the specified UserSignup value, so even
// if an error is returned by this function the caller should still process changes to it
func (s *ServiceImpl) VerifyCode(ctx *gin.Context, userID string, code string) (verificationErr error) {

	signup, verificationErr := s.Services().SignupService().GetUserSignup(userID)
	if verificationErr != nil {
		log.Error(ctx, verificationErr, "error retrieving usersignup")
		return
	}

	now := time.Now()

	// If 24 hours has passed since the verification timestamp, then reset the timestamp and verification attempts
	ts, parseErr := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserSignupVerificationTimestampAnnotationKey])
	if parseErr != nil || (parseErr == nil && now.After(ts.Add(24*time.Hour))) {
		// Set a new timestamp
		signup.Annotations[v1alpha1.UserSignupVerificationTimestampAnnotationKey] = now.Format(TimestampLayout)
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = "0"
	}

	attemptsMade, convErr := strconv.Atoi(signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey])
	if convErr != nil {
		// We shouldn't get an error here, but if we do, we will set verification attempts to max allowed
		// so that we at least now have a valid value, and let the workflow continue to the
		// subsequent attempts check
		attemptsMade = s.config.GetVerificationAttemptsAllowed()
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
	}

	// If the user has made more attempts than is allowed per day, return an error
	if attemptsMade >= s.config.GetVerificationAttemptsAllowed() {
		verificationErr = errors.NewTooManyRequestsError("too many verification attempts", "")
	}

	if verificationErr == nil {
		// Parse the verification expiry timestamp
		exp, parseErr := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserVerificationExpiryAnnotationKey])
		if parseErr != nil {
			// If the verification expiry timestamp is corrupt or missing, then return an error
			verificationErr = errors.NewInternalError(parseErr, "error parsing expiry timestamp")
		} else if now.After(exp) {
			// If it is now past the expiry timestamp for the verification code, return a 403 Forbidden error
			verificationErr = errors.NewForbiddenError("expired", "verification code expired")
		}
	}

	if verificationErr == nil {
		if code != signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey] {
			// The code doesn't match
			attemptsMade++
			signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
			verificationErr = errors.NewForbiddenError("invalid code", "the provided code is invalid")
		}
	}

	if verificationErr == nil {
		// If the code matches then set VerificationRequired to false, reset other verification annotations
		signup.Spec.VerificationRequired = false
		delete(signup.Annotations, v1alpha1.UserSignupVerificationCodeAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserVerificationAttemptsAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserSignupVerificationCounterAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserSignupVerificationTimestampAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserSignupVerificationInitTimestampAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserVerificationExpiryAnnotationKey)
	} else {
		log.Error(ctx, verificationErr, "error validating verification code")
	}

	// Update changes made to the UserSignup
	_, updateErr := s.Services().SignupService().UpdateUserSignup(signup)
	if updateErr != nil {
		log.Error(ctx, updateErr, "error updating UserSignup")
		verificationErr = updateErr
	}

	return
}
