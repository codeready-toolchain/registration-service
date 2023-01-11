package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/verification/sender"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	codeCharset = "0123456789"
	codeLength  = 6

	TimestampLayout = "2006-01-02T15:04:05.000Z07:00"
)

// ServiceImpl represents the implementation of the verification service.
type ServiceImpl struct { // nolint:revive
	base.BaseService
	HTTPClient          *http.Client
	NotificationService sender.NotificationSender
}

type VerificationServiceOption func(svc *ServiceImpl)

// NewVerificationService creates a service object for performing user verification
func NewVerificationService(context servicecontext.ServiceContext, opts ...VerificationServiceOption) service.VerificationService {
	s := &ServiceImpl{
		BaseService: base.NewBaseService(context),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.NotificationService = sender.CreateNotificationSender(s.HTTPClient)

	return s
}

// InitVerification sends a verification message to the specified user, using the Twilio service.  If successful,
// the user will receive a verification SMS.  The UserSignup resource is updated with a number of annotations in order
// to manage the phone verification process and protect against system abuse.
func (s *ServiceImpl) InitVerification(ctx *gin.Context, userID, username, e164PhoneNumber string) error {
	signup, err := s.Services().SignupService().GetUserSignupFromIdentifier(userID, username)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(ctx, err, "usersignup not found")
			return crterrors.NewNotFoundError(err, "usersignup not found")
		}
		log.Error(ctx, err, "error retrieving usersignup")
		return crterrors.NewInternalError(err, fmt.Sprintf("error retrieving usersignup: %s", userID))
	}
	labelValues := map[string]string{}
	annotationValues := map[string]string{}

	// check that verification is required before proceeding
	if !states.VerificationRequired(signup) {
		log.Info(ctx, fmt.Sprintf("phone verification attempted for user without verification requirement: %s", userID))
		return crterrors.NewBadRequest("forbidden request", "verification code will not be sent")
	}

	// Check if the provided phone number is already being used by another user
	err = s.Services().SignupService().PhoneNumberAlreadyInUse(userID, username, e164PhoneNumber)
	if err != nil {
		e := &crterrors.Error{}
		switch {
		case errors.As(err, &e) && e.Code == http.StatusForbidden:
			log.Errorf(ctx, err, "phone number already in use, cannot register using phone number: %s", e164PhoneNumber)
			return crterrors.NewForbiddenError("phone number already in use", fmt.Sprintf("cannot register using phone number: %s", e164PhoneNumber))
		default:
			log.Error(ctx, err, "error while looking up users by phone number")
			return crterrors.NewInternalError(err, "could not lookup users by phone number")
		}
	}

	// calculate the phone number hash
	phoneHash := hash.EncodeString(e164PhoneNumber)

	labelValues[toolchainv1alpha1.UserSignupUserPhoneHashLabelKey] = phoneHash

	// read the current time
	now := time.Now()

	// If 24 hours has passed since the verification timestamp, then reset the timestamp and verification attempts
	ts, parseErr := time.Parse(TimestampLayout, signup.Annotations[toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey])
	if parseErr != nil || now.After(ts.Add(24*time.Hour)) {
		// Set a new timestamp
		annotationValues[toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey] = now.Format(TimestampLayout)
		annotationValues[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey] = "0"
	}

	// get the verification counter (i.e. the number of times the user has initiated phone verification within
	// the last 24 hours)
	verificationCounter := signup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey]
	var counter int
	cfg := configuration.GetRegistrationServiceConfig()

	dailyLimit := cfg.Verification().DailyLimit()
	if verificationCounter != "" {
		counter, err = strconv.Atoi(verificationCounter)
		if err != nil {
			// We shouldn't get an error here, but if we do, we should probably set verification counter to the daily
			// limit so that we at least now have a valid value
			log.Error(ctx, err, fmt.Sprintf("error converting annotation [%s] value [%s] to integer, on UserSignup: [%s]",
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey,
				signup.Annotations[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey], signup.Name))
			annotationValues[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(dailyLimit)
			counter = dailyLimit
		}
	}

	var initError error
	// check if counter has exceeded the limit of daily limit - if at limit error out
	if counter >= dailyLimit {
		log.Error(ctx, err, fmt.Sprintf("%d attempts made. the daily limit of %d has been exceeded", counter, dailyLimit))
		initError = crterrors.NewForbiddenError("daily limit exceeded", "cannot generate new verification code")
	}

	if initError == nil {
		// generate verification code
		verificationCode, err := generateVerificationCode()
		if err != nil {
			return crterrors.NewInternalError(err, "error while generating verification code")
		}
		// set the usersignup annotations
		annotationValues[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey] = "0"
		annotationValues[toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey] = strconv.Itoa(counter + 1)
		annotationValues[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey] = verificationCode
		annotationValues[toolchainv1alpha1.UserVerificationExpiryAnnotationKey] = now.Add(
			time.Duration(cfg.Verification().CodeExpiresInMin()) * time.Minute).Format(TimestampLayout)

		// Generate the verification message with the new verification code
		content := fmt.Sprintf(cfg.Verification().MessageTemplate(), verificationCode)

		err = s.NotificationService.SendNotification(ctx, content, e164PhoneNumber)
		if err != nil {
			log.Error(ctx, err, "error while sending notification")

			// If we get an error here then just die, don't bother updating the UserSignup
			return crterrors.NewInternalError(err, "error while sending verification code")
		}
	}

	doUpdate := func() error {
		signup, err := s.Services().SignupService().GetUserSignupFromIdentifier(userID, username)
		if err != nil {
			return err
		}
		if signup.Labels == nil {
			signup.Labels = map[string]string{}
		}

		for k, v := range labelValues {
			signup.Labels[k] = v
		}

		for k, v := range annotationValues {
			signup.Annotations[k] = v
		}
		_, err = s.Services().SignupService().UpdateUserSignup(signup)
		if err != nil {
			return err
		}

		return nil
	}

	updateErr := pollUpdateSignup(ctx, doUpdate)
	if updateErr != nil {
		return updateErr
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

// VerifyPhoneCode validates the user's phone verification code.  It updates the specified UserSignup value, so even
// if an error is returned by this function the caller should still process changes to it
func (s *ServiceImpl) VerifyPhoneCode(ctx *gin.Context, userID, username, code string) (verificationErr error) {

	cfg := configuration.GetRegistrationServiceConfig()
	// If we can't even find the UserSignup, then die here
	signup, lookupErr := s.Services().SignupService().GetUserSignupFromIdentifier(userID, username)
	if lookupErr != nil {
		if apierrors.IsNotFound(lookupErr) {
			log.Error(ctx, lookupErr, "usersignup not found")
			return crterrors.NewNotFoundError(lookupErr, "user not found")
		}
		log.Error(ctx, lookupErr, "error retrieving usersignup")
		return crterrors.NewInternalError(lookupErr, fmt.Sprintf("error retrieving usersignup: %s", userID))
	}

	annotationValues := map[string]string{}
	annotationsToDelete := []string{}
	unsetVerificationRequired := false

	err := s.Services().SignupService().PhoneNumberAlreadyInUse(userID, username, signup.Labels[toolchainv1alpha1.UserSignupUserPhoneHashLabelKey])
	if err != nil {
		log.Error(ctx, err, "phone number to verify already in use")
		return crterrors.NewBadRequest("phone number already in use",
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
		attemptsMade = cfg.Verification().AttemptsAllowed()
		annotationValues[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
	}

	// If the user has made more attempts than is allowed per generated verification code, return an error
	if attemptsMade >= cfg.Verification().AttemptsAllowed() {
		verificationErr = crterrors.NewTooManyRequestsError("too many verification attempts", "")
	}

	if verificationErr == nil {
		// Parse the verification expiry timestamp
		exp, parseErr := time.Parse(TimestampLayout, signup.Annotations[toolchainv1alpha1.UserVerificationExpiryAnnotationKey])
		if parseErr != nil {
			// If the verification expiry timestamp is corrupt or missing, then return an error
			verificationErr = crterrors.NewInternalError(parseErr, "error parsing expiry timestamp")
		} else if now.After(exp) {
			// If it is now past the expiry timestamp for the verification code, return a 403 Forbidden error
			verificationErr = crterrors.NewForbiddenError("expired", "verification code expired")
		}
	}

	if verificationErr == nil {
		if code != signup.Annotations[toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey] {
			// The code doesn't match
			attemptsMade++
			annotationValues[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)
			verificationErr = crterrors.NewForbiddenError("invalid code", "the provided code is invalid")
		}
	}

	if verificationErr == nil {
		// If the code matches then set VerificationRequired to false, reset other verification annotations
		unsetVerificationRequired = true
		annotationsToDelete = append(annotationsToDelete, toolchainv1alpha1.UserSignupVerificationCodeAnnotationKey)
		annotationsToDelete = append(annotationsToDelete, toolchainv1alpha1.UserVerificationAttemptsAnnotationKey)
		annotationsToDelete = append(annotationsToDelete, toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey)
		annotationsToDelete = append(annotationsToDelete, toolchainv1alpha1.UserSignupVerificationInitTimestampAnnotationKey)
		annotationsToDelete = append(annotationsToDelete, toolchainv1alpha1.UserVerificationExpiryAnnotationKey)
	} else {
		log.Error(ctx, verificationErr, "error validating verification code")
	}

	doUpdate := func() error {
		signup, err := s.Services().SignupService().GetUserSignupFromIdentifier(userID, username)
		if err != nil {
			return err
		}

		if unsetVerificationRequired {
			states.SetVerificationRequired(signup, false)
		}

		for k, v := range annotationValues {
			signup.Annotations[k] = v
		}

		for _, annotationName := range annotationsToDelete {
			delete(signup.Annotations, annotationName)
		}

		_, err = s.Services().SignupService().UpdateUserSignup(signup)
		if err != nil {
			return err
		}

		return nil
	}

	updateErr := pollUpdateSignup(ctx, doUpdate)
	if updateErr != nil {
		return updateErr
	}

	return
}

// VerifyActivationCode verifies the activation code:
// - checks that the SocialEvent resource named after the activation code exists
// - checks that the SocialEvent has enough capacity to approve the user
func (s *ServiceImpl) VerifyActivationCode(ctx *gin.Context, userID, username, code string) error {
	log.Infof(ctx, "verifying activation code '%s'", code)
	// look-up the UserSignup
	signup, err := s.Services().SignupService().GetUserSignupFromIdentifier(userID, username)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return crterrors.NewNotFoundError(err, "user not found")
		}
		return crterrors.NewInternalError(err, fmt.Sprintf("error retrieving usersignup: %s", userID))
	}
	annotationValues := map[string]string{}
	annotationsToDelete := []string{}
	unsetVerificationRequired := false

	defer func() {
		doUpdate := func() error {
			signup, err := s.Services().SignupService().GetUserSignupFromIdentifier(userID, username)
			if err != nil {
				return err
			}
			if unsetVerificationRequired {
				states.SetVerificationRequired(signup, false)
			}
			if signup.Annotations == nil {
				signup.Annotations = map[string]string{}
			}
			for k, v := range annotationValues {
				signup.Annotations[k] = v
			}
			for _, annotationName := range annotationsToDelete {
				delete(signup.Annotations, annotationName)
			}
			// also, label the UserSignup with the name of the SocialEvent (ie, the activation code)
			if signup.Labels == nil {
				signup.Labels = map[string]string{}
			}
			signup.Labels[toolchainv1alpha1.SocialEventUserSignupLabelKey] = code
			_, err = s.Services().SignupService().UpdateUserSignup(signup)
			if err != nil {
				return err
			}

			return nil
		}
		if err := pollUpdateSignup(ctx, doUpdate); err != nil {
			log.Errorf(ctx, err, "unable to update user signup after validating activation code")
		}
	}()

	attemptsMade, err := checkAttempts(signup)
	if err != nil {
		return err
	}
	attemptsMade++
	annotationValues[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey] = strconv.Itoa(attemptsMade)

	// look-up the SocialEvent
	event, err := s.CRTClient().V1Alpha1().SocialEvents().Get(code)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// a SocialEvent was not found for the provided code
			return crterrors.NewForbiddenError("invalid code", "the provided code is invalid")
		}
		return crterrors.NewInternalError(err, fmt.Sprintf("error retrieving event '%s'", code))
	}
	// if there is room for the user and if the "time window" to signup is valid
	now := metav1.NewTime(time.Now())
	log.Infof(ctx, "verifying activation code '%s': event.Status.ActivationCount=%d, event.Spec.MaxAttendees=%s, event.Spec.StartTime=%s, event.Spec.EndTime=%s", code, strconv.Itoa(event.Status.ActivationCount), strconv.Itoa(event.Spec.MaxAttendees), event.Spec.StartTime.Format("2006-01-02:03:04:05"), event.Spec.EndTime.Format("2006-01-02:03:04:05"))

	if event.Status.ActivationCount >= event.Spec.MaxAttendees {
		return crterrors.NewForbiddenError("invalid code", "the event is full")
	} else if event.Spec.StartTime.After(now.Time) || event.Spec.EndTime.Before(&now) {
		log.Infof(ctx, "the event with code '%s' has not started yet or is already past", code)
		return crterrors.NewForbiddenError("invalid code", "the provided code is invalid")
	}
	log.Infof(ctx, "approving user signup request with activation code '%s'", code)
	// If the activation code is acceptable then set `VerificationRequired` state to false and reset other verification annotations
	unsetVerificationRequired = true
	annotationsToDelete = append(annotationsToDelete, toolchainv1alpha1.UserVerificationAttemptsAnnotationKey)
	return nil
}

func checkAttempts(signup *toolchainv1alpha1.UserSignup) (int, error) {
	cfg := configuration.GetRegistrationServiceConfig()
	v, found := signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey]
	if !found || v == "" {
		return 0, nil
	}
	attemptsMade, err := strconv.Atoi(v)
	if err != nil {
		return -1, crterrors.NewInternalError(err, fmt.Sprintf("error converting annotation [%s] value [%s] to integer, on UserSignup: [%s]",
			toolchainv1alpha1.UserVerificationAttemptsAnnotationKey,
			signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey], signup.Name))
	}
	// If the user has made more attempts than is allowed per generated verification code, return an error
	if attemptsMade >= cfg.Verification().AttemptsAllowed() {
		return attemptsMade, crterrors.NewTooManyRequestsError("too many verification attempts", signup.Annotations[toolchainv1alpha1.UserVerificationAttemptsAnnotationKey])
	}
	return attemptsMade, nil
}

func pollUpdateSignup(ctx *gin.Context, updater func() error) error {
	// Attempt to execute an update function, retrying a number of times if the update fails
	attempts := 0
	for {
		attempts++

		// Attempt the update
		updateErr := updater()

		// If there was an error, then only log it for now
		if updateErr != nil {
			log.Error(ctx, updateErr, fmt.Sprintf("error while executing updating, attempt #%d", attempts))
		} else {
			// Otherwise if there was no error executing the update, then break here
			break
		}

		// If we've exceeded the number of attempts, then return a useful error to the user.  We won't return the actual
		// error to the user here, as we've already logged it
		if attempts > 4 {
			return crterrors.NewInternalError(errors.New("there was an error while updating your account - please wait a moment before trying again."+
				" If this error persists, please contact the Developer Sandbox team at devsandbox@redhat.com for assistance"),
				"error while verifying phone code")
		}
	}

	return nil
}
