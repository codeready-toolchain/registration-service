package signup

import (
	"fmt"
	"strconv"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetAndValidateSocialEvent returns a SocialEvent with the given name.
// If the event is already full, not yet started, already finished, or not found then it returns error
func GetAndValidateSocialEvent(ctx *gin.Context, cl namespaced.Client, code string) (*toolchainv1alpha1.SocialEvent, error) {
	// look-up the SocialEvent
	event := &toolchainv1alpha1.SocialEvent{}
	if err := cl.Get(ctx, cl.NamespacedName(code), event); err != nil {
		if apierrors.IsNotFound(err) {
			// a SocialEvent was not found for the provided code
			return nil, crterrors.NewForbiddenError("invalid code", "the provided code is invalid")
		}
		return nil, crterrors.NewInternalError(err, fmt.Sprintf("error retrieving event '%s'", code))
	}
	// if there is room for the user and if the "time window" to signup is valid
	now := metav1.NewTime(time.Now())
	log.Infof(ctx, "verifying activation code '%s': event.Status.ActivationCount=%s, event.Spec.MaxAttendees=%s, event.Spec.StartTime=%s, event.Spec.EndTime=%s",
		code, strconv.Itoa(event.Status.ActivationCount), strconv.Itoa(event.Spec.MaxAttendees), event.Spec.StartTime.Format("2006-01-02:03:04:05"), event.Spec.EndTime.Format("2006-01-02:03:04:05"))

	if event.Status.ActivationCount >= event.Spec.MaxAttendees {
		return nil, crterrors.NewForbiddenError("invalid code", "the event is full")
	} else if event.Spec.StartTime.After(now.Time) {
		log.Infof(ctx, "the event with code '%s' has not started yet", code)
		return nil, crterrors.NewForbiddenError("invalid code", "the provided code is not valid yet")
	} else if event.Spec.EndTime.Before(&now) {
		log.Infof(ctx, "the event with code '%s' is already past", code)
		return nil, crterrors.NewForbiddenError("invalid code", "the provided code has expired")
	}
	return event, nil
}

// UpdateUserSignupWithSocialEvent updates fields in the userSignup with values from the given SocialEvent
func UpdateUserSignupWithSocialEvent(event *toolchainv1alpha1.SocialEvent, userSignup *toolchainv1alpha1.UserSignup) {
	if !event.Spec.VerificationRequired {
		states.SetApprovedManually(userSignup, true)
	}
	// make sure that the user is not deactivated
	states.SetDeactivated(userSignup, false)

	// label the UserSignup with the name of the SocialEvent (ie, the activation code)
	if userSignup.Labels == nil {
		userSignup.Labels = map[string]string{}
	}
	userSignup.Labels[toolchainv1alpha1.SocialEventUserSignupLabelKey] = event.Name
	if event.Spec.TargetCluster != "" {
		userSignup.Spec.TargetCluster = event.Spec.TargetCluster
	}
}
