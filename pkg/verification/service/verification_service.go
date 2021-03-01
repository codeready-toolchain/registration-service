package service

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/kevinburke/twilio-go"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// InitVerification sends a verification message to the specified user, using the Twilio service.  If successful,
// the user will receive a verification SMS.  The UserSignup resource is updated with a number of annotations in order
// to manage the phone verification process and protect against system abuse.
func (s *ServiceImpl) InitVerification(ctx *gin.Context, userID string, e164PhoneNumber string) error {
	signup, err := s.Services().SignupService().GetUserSignup(userID)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Error(ctx, err, "usersignup not found")
			return errors.NewNotFound(schema.GroupResource{Group: signup.APIVersion, Resource: signup.Kind}, userID)
		}
		log.Error(ctx, err, "error retrieving usersignup")
		return err
	}

	// check that verification is required before proceeding
	if signup.Spec.VerificationRequired == false {
		log.Info(ctx, fmt.Sprintf("phone verification attempted for user without verification requirement: %s", userID))
		return errors.NewBadRequest("forbidden request:verification code will not be sent")
	}

	// Check if the provided phone number is already being used by another user
	err = s.Services().SignupService().PhoneNumberAlreadyInUse(userID, e164PhoneNumber)
	if err != nil {
		if errors.IsForbidden(err) {
			log.Errorf(ctx, err, "phone number already in use, cannot register using phone number: %s", e164PhoneNumber)
			return errors.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("phone number already in use: cannot register using phone number: %s", e164PhoneNumber))
		}
		log.Error(ctx, err, "error while looking up users by phone number")
		return err
	}

	// calculate the phone number hash
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(e164PhoneNumber))
	phoneHash := hex.EncodeToString(md5hash.Sum(nil))

	if signup.Labels == nil {
		signup.Labels = map[string]string{}
	}
	signup.Labels[v1alpha1.UserSignupUserPhoneHashLabelKey] = phoneHash

	// read the current time
	now := time.Now()

	// If 24 hours has passed since the verification timestamp, then reset the timestamp and verification attempts
	ts, parseErr := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserSignupVerificationInitTimestampAnnotationKey])
	if parseErr != nil || now.After(ts.Add(24*time.Hour)) {
		// Set a new timestamp
		signup.Annotations[v1alpha1.UserSignupVerificationInitTimestampAnnotationKey] = now.Format(TimestampLayout)
		signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey] = "0"
	}

	// get the verification counter (i.e. the number of times the user has initiated phone verification within
	// the last 24 hours)
	verificationCounter := signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey]
	var counter int
	dailyLimit := s.config.GetVerificationDailyLimit()
	if verificationCounter != "" {
		counter, err = strconv.Atoi(verificationCounter)
		if err != nil {
			// We shouldn't get an error here, but if we do, we should probably set verification counter to the daily
			// limit so that we at least now have a valid value
			log.Error(ctx, err, fmt.Sprintf("error converting annotation [%s] value [%s] to integer, on UserSignup: [%s]",
				v1alpha1.UserSignupVerificationCounterAnnotationKey,
				signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey], signup.Name))
			signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(dailyLimit)
			counter = dailyLimit
		}
	}

	var initError error
	// check if counter has exceeded the limit of daily limit - if at limit error out
	if counter >= dailyLimit {
		log.Error(ctx, err, fmt.Sprintf("%d attempts made. the daily limit of %d has been exceeded", counter, dailyLimit))
		initError = errors.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("daily limit exceeded: cannot generate new verification code"))
	}

	if initError == nil {
		// generate verification code
		verificationCode := generateVerificationCode()

		// set the usersignup annotations
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = "0"
		signup.Annotations[v1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(counter + 1)
		signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey] = verificationCode
		signup.Annotations[v1alpha1.UserVerificationExpiryAnnotationKey] = now.Add(
			time.Duration(s.config.GetVerificationCodeExpiresInMin()) * time.Minute).Format(TimestampLayout)

		// Generate the verification message with the new verification code
		content := fmt.Sprintf(s.config.GetVerificationMessageTemplate(), verificationCode)
		client := twilio.NewClient(s.config.GetTwilioAccountSID(), s.config.GetTwilioAuthToken(), s.HttpClient)
		msg, err := client.Messages.SendMessage(s.config.GetTwilioFromNumber(), e164PhoneNumber, content, nil)
		if err != nil {
			if msg != nil {
				log.Error(ctx, err, fmt.Sprintf("error while sending, code: %d message: %s", msg.ErrorCode, msg.ErrorMessage))
			} else {
				log.Error(ctx, err, "unknown error while sending")
			}

			// If we get an error here then just die, don't bother updating the UserSignup
			return err
		}
	}

	_, updateErr := s.Services().SignupService().UpdateUserSignup(signup)
	if updateErr != nil {
		log.Error(ctx, err, "error while updating UserSignup resource")
		return updateErr
	}

	return initError
}

func generateVerificationCode() string {
	buf := make([]byte, codeLength)
	rand.Read(buf)

	charsetLen := len(codeCharset)
	for i := 0; i < codeLength; i++ {
		buf[i] = codeCharset[int(buf[i])%charsetLen]
	}

	return string(buf)
}

// VerifyCode validates the user's phone verification code.  It updates the specified UserSignup value, so even
// if an error is returned by this function the caller should still process changes to it
func (s *ServiceImpl) VerifyCode(ctx *gin.Context, userID string, code string) error {

	// If we can't even find the UserSignup, then die here
	signup, lookupErr := s.Services().SignupService().GetUserSignup(userID)
	if lookupErr != nil {
		if errors.IsNotFound(lookupErr) {
			log.Error(ctx, lookupErr, "usersignup not found")
			// TODO: fix this
			return errors.NewNotFound(schema.GroupResource{Group: "toolchain.dev.openshift.com/v1alpha1", Resource: "UserSignup"}, userID)
		}
		log.Error(ctx, lookupErr, "error retrieving usersignup")
		return lookupErr
	}

	err := s.Services().SignupService().PhoneNumberAlreadyInUse(userID, signup.Labels[v1alpha1.UserSignupUserPhoneHashLabelKey])
	if err != nil {
		log.Error(ctx, err, "phone number to verify already in use")
		return errors.NewBadRequest("phone number already in use:the phone number provided for this signup is already in use by an active account")
	}

	now := time.Now()

	attemptsMade, convErr := strconv.Atoi(signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey])
	if convErr != nil {
		// We shouldn't get an error here, but if we do, we will set verification attempts to max allowed
		// so that we at least now have a valid value, and let the workflow continue to the
		// subsequent attempts check
		log.Error(ctx, convErr, fmt.Sprintf("error converting annotation [%s] value [%s] to integer, on UserSignup: [%s]",
			v1alpha1.UserVerificationAttemptsAnnotationKey,
			signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey], signup.Name))
		attemptsMade = s.config.GetVerificationAttemptsAllowed()
		signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
	}

	var verificationErr error

	// If the user has made more attempts than is allowed per generated verification code, return an error
	if attemptsMade >= s.config.GetVerificationAttemptsAllowed() {
		verificationErr = errors.NewTooManyRequestsError("too many verification attempts")
	}

	// Parse the verification expiry timestamp
	if verificationErr == nil {
		exp, parseErr := time.Parse(TimestampLayout, signup.Annotations[v1alpha1.UserVerificationExpiryAnnotationKey])
		if parseErr != nil {
			// If the verification expiry timestamp is corrupt or missing, then return an error
			verificationErr = parseErr
		} else if now.After(exp) {
			// If it is now past the expiry timestamp for the verification code, return a 403 Forbidden error
			verificationErr = errors.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("expired: verification code expired"))
		}
	}

	if verificationErr == nil {
		if code != signup.Annotations[v1alpha1.UserSignupVerificationCodeAnnotationKey] {
			// The code doesn't match
			attemptsMade++
			signup.Annotations[v1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
			verificationErr = errors.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("invalid code: the provided code is invalid"))
		}
	}
	// If the code matches then set VerificationRequired to false, reset other verification annotations
	if verificationErr == nil {
		signup.Spec.VerificationRequired = false
		delete(signup.Annotations, v1alpha1.UserSignupVerificationCodeAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserVerificationAttemptsAnnotationKey)
		delete(signup.Annotations, v1alpha1.UserSignupVerificationCounterAnnotationKey)
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

	return verificationErr
}
