package service

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/codeready-toolchain/toolchain-common/pkg/states"

	"github.com/kevinburke/twilio-go"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"

	"github.com/codeready-toolchain/registration-service/pkg/errors"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	codeCharset = "0123456789"
	codeLength  = 6

	TimestampLayout = "2006-01-02T15:04:05.000Z07:00"
)

// ServiceConfiguration represents the config used for the verification service.
type ServiceConfiguration interface { // nolint: golint
	GetTwilioAccountSID() string
	GetTwilioAuthToken() string
	GetTwilioFromNumber() string
	GetVerificationMessageTemplate() string
	GetVerificationAttemptsAllowed() int
	GetVerificationDailyLimit() int
	GetVerificationCodeExpiresInMin() int
}

// ServiceImpl represents the implementation of the verification service.
type ServiceImpl struct { // nolint: golint
	base.BaseService
	config     ServiceConfiguration
	HTTPClient *http.Client
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

// InitVerification sends a verification message to the specified user, using the Twilio service.  If successful,
// the user will receive a verification SMS.  The UserSignup resource is updated with a number of annotations in order
// to manage the phone verification process and protect against system abuse.
func (s *ServiceImpl) InitVerification(ctx *gin.Context, userID string, e164PhoneNumber string) error {
	signup, err := s.Services().SignupService().GetUserSignup(userID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(ctx, err, "usersignup not found")
			return errors.NewNotFoundError(err, "usersignup not found")
		}
		log.Error(ctx, err, "error retrieving usersignup")
		return errors.NewInternalError(err, fmt.Sprintf("error retrieving usersignup: %s", userID))
	}

	// check that verification is required before proceeding
	if !states.VerificationRequired(signup) {
		log.Info(ctx, fmt.Sprintf("phone verification attempted for user without verification requirement: %s", userID))
		return errors.NewBadRequest("forbidden request", "verification code will not be sent")
	}

	// Check if the provided phone number is already being used by another user
	err = s.Services().SignupService().PhoneNumberAlreadyInUse(userID, e164PhoneNumber)
	if err != nil {
		switch t := err.(type) {
		case *errors.Error:
			if t.Code == http.StatusForbidden {
				log.Errorf(ctx, err, "phone number already in use, cannot register using phone number: %s", e164PhoneNumber)
				return errors.NewForbiddenError("phone number already in use", fmt.Sprintf("cannot register using phone number: %s", e164PhoneNumber))
			}
		default:
			log.Error(ctx, err, "error while looking up users by phone number")
			return errors.NewInternalError(err, "could not lookup users by phone number")
		}
	}

	// calculate the phone number hash
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(e164PhoneNumber))
	phoneHash := hex.EncodeToString(md5hash.Sum(nil))

	if signup.Labels == nil {
		signup.Labels = map[string]string{}
	}
	signup.Labels[toolchainv1alpha1.UserSignupUserPhoneHashLabelKey] = phoneHash

	// read the current time
	now := time.Now()

	// If 24 hours has passed since the verification timestamp, then reset the timestamp and verification attempts
	ts, parseErr := time.Parse(TimestampLayout, signup.Annotations[toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey])
	if parseErr != nil || now.After(ts.Add(24*time.Hour)) {
		// Set a new timestamp
		signup.Annotations[toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey] = now.Format(TimestampLayout)
		signup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey] = "0"
	}

	// get the verification counter (i.e. the number of times the user has initiated phone verification within
	// the last 24 hours)
	verificationCounter := signup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey]
	var counter int
	dailyLimit := s.config.GetVerificationDailyLimit()
	if verificationCounter != "" {
		counter, err = strconv.Atoi(verificationCounter)
		if err != nil {
			// We shouldn't get an error here, but if we do, we should probably set verification counter to the daily
			// limit so that we at least now have a valid value
			log.Error(ctx, err, fmt.Sprintf("error converting annotation [%s] value [%s] to integer, on UserSignup: [%s]",
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey,
				signup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey], signup.Name))
			signup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(dailyLimit)
			counter = dailyLimit
		}
	}

	var initError error
	// check if counter has exceeded the limit of daily limit - if at limit error out
	if counter >= dailyLimit {
		log.Error(ctx, err, fmt.Sprintf("%d attempts made. the daily limit of %d has been exceeded", counter, dailyLimit))
		initError = errors.NewForbiddenError("daily limit exceeded", "cannot generate new verification code")
	}

	if initError == nil {
		// generate verification code
		verificationCode, err := generateVerificationCode()
		if err != nil {
			return errors.NewInternalError(err, "error while generating verification code")
		}
		// set the usersignup annotations
		signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey] = "0"
		signup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(counter + 1)
		signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey] = verificationCode
		signup.Annotations[toolchainv1alpha1.UserVerificationExpiryAnnotationKey] = now.Add(
			time.Duration(s.config.GetVerificationCodeExpiresInMin()) * time.Minute).Format(TimestampLayout)

		// Generate the verification message with the new verification code
		content := fmt.Sprintf(s.config.GetVerificationMessageTemplate(), verificationCode)
		client := twilio.NewClient(s.config.GetTwilioAccountSID(), s.config.GetTwilioAuthToken(), s.HTTPClient)
		msg, err := client.Messages.SendMessage(s.config.GetTwilioFromNumber(), e164PhoneNumber, content, nil)
		if err != nil {
			if msg != nil {
				log.Error(ctx, err, fmt.Sprintf("error while sending, code: %d message: %s", msg.ErrorCode, msg.ErrorMessage))
			} else {
				log.Error(ctx, err, "unknown error while sending")
			}

			// If we get an error here then just die, don't bother updating the UserSignup
			return errors.NewInternalError(err, "error while sending verification code")
		}
	}

	_, updateErr := s.Services().SignupService().UpdateUserSignup(signup)
	if updateErr != nil {
		log.Error(ctx, err, "error while updating UserSignup resource")
		return errors.NewInternalError(updateErr, "error while updating UserSignup resource")
	}

	return initError
}

func generateVerificationCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
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

	// If we can't even find the UserSignup, then die here
	signup, lookupErr := s.Services().SignupService().GetUserSignup(userID)
	if lookupErr != nil {
		if apierrors.IsNotFound(lookupErr) {
			log.Error(ctx, lookupErr, "usersignup not found")
			return errors.NewNotFoundError(lookupErr, "user not found")
		}
		log.Error(ctx, lookupErr, "error retrieving usersignup")
		return errors.NewInternalError(lookupErr, fmt.Sprintf("error retrieving usersignup: %s", userID))
	}

	err := s.Services().SignupService().PhoneNumberAlreadyInUse(userID, signup.Labels[toolchainv1alpha1.UserSignupUserPhoneHashLabelKey])
	if err != nil {
		log.Error(ctx, err, "phone number to verify already in use")
		return errors.NewBadRequest("phone number already in use",
			"the phone number provided for this signup is already in use by an active account")
	}

	now := time.Now()

	attemptsMade, convErr := strconv.Atoi(signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey])
	if convErr != nil {
		// We shouldn't get an error here, but if we do, we will set verification attempts to max allowed
		// so that we at least now have a valid value, and let the workflow continue to the
		// subsequent attempts check
		log.Error(ctx, convErr, fmt.Sprintf("error converting annotation [%s] value [%s] to integer, on UserSignup: [%s]",
			toolchainv1alpha1.UserVerificationAttemptsAnnotationKey,
			signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey], signup.Name))
		attemptsMade = s.config.GetVerificationAttemptsAllowed()
		signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
	}

	// If the user has made more attempts than is allowed per generated verification code, return an error
	if attemptsMade >= s.config.GetVerificationAttemptsAllowed() {
		verificationErr = errors.NewTooManyRequestsError("too many verification attempts", "")
	}

	if verificationErr == nil {
		// Parse the verification expiry timestamp
		exp, parseErr := time.Parse(TimestampLayout, signup.Annotations[toolchainv1alpha1.UserVerificationExpiryAnnotationKey])
		if parseErr != nil {
			// If the verification expiry timestamp is corrupt or missing, then return an error
			verificationErr = errors.NewInternalError(parseErr, "error parsing expiry timestamp")
		} else if now.After(exp) {
			// If it is now past the expiry timestamp for the verification code, return a 403 Forbidden error
			verificationErr = errors.NewForbiddenError("expired", "verification code expired")
		}
	}

	if verificationErr == nil {
		if code != signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey] {
			// The code doesn't match
			attemptsMade++
			signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
			verificationErr = errors.NewForbiddenError("invalid code", "the provided code is invalid")
		}
	}

	if verificationErr == nil {
		// If the code matches then set VerificationRequired to false, reset other verification annotations
		states.SetVerificationRequired(signup, false)
		delete(signup.Annotations, toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey)
		delete(signup.Annotations, toolchainv1alpha1.UserVerificationAttemptsAnnotationKey)
		delete(signup.Annotations, toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey)
		delete(signup.Annotations, toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey)
		delete(signup.Annotations, toolchainv1alpha1.UserVerificationExpiryAnnotationKey)
	} else {
		log.Error(ctx, verificationErr, "error validating verification code")
	}

	// Update changes made to the UserSignup
	_, updateErr := s.Services().SignupService().UpdateUserSignup(signup)
	if updateErr != nil {
		log.Error(ctx, updateErr, "error updating UserSignup")
		verificationErr = errors.NewInternalError(updateErr, "error updating UserSignup")
	}

	return
}
