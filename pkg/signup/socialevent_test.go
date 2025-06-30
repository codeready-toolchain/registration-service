package signup

import (
	"net/http/httptest"
	"testing"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	testsocialevent "github.com/codeready-toolchain/toolchain-common/pkg/test/socialevent"
	"github.com/codeready-toolchain/toolchain-common/pkg/test/usersignup"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSocialEvent(t *testing.T) {
	// given
	log.Init("social-code-testing")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	t.Run("success", func(t *testing.T) {
		// given
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event1",
			testsocialevent.WithTargetCluster("member"),
			testsocialevent.WithMaxAttendees(100),
			testsocialevent.WithActivationCount(99),
			testsocialevent.WithStartTime(time.Now().Add(-time.Hour)),
			testsocialevent.WithEndTime(time.Now().Add(time.Hour)))
		nsdClient := namespaced.NewClient(commontest.NewFakeClient(t, event), commontest.HostOperatorNs)

		// when
		event, err := GetAndValidateSocialEvent(ctx, nsdClient, "event1")

		// then
		require.NoError(t, err)
		require.NotNil(t, event)
		assert.Equal(t, "deactivate30", event.Spec.UserTier)
		assert.Equal(t, "base1ns", event.Spec.SpaceTier)
		assert.Equal(t, 100, event.Spec.MaxAttendees)
		assert.Equal(t, 99, event.Status.ActivationCount)
		assert.Equal(t, "member", event.Spec.TargetCluster)
	})

	t.Run("when event is full", func(t *testing.T) {
		// given
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event1",
			testsocialevent.WithMaxAttendees(100),
			testsocialevent.WithActivationCount(100))
		nsdClient := namespaced.NewClient(commontest.NewFakeClient(t, event), commontest.HostOperatorNs)

		// when
		event, err := GetAndValidateSocialEvent(ctx, nsdClient, "event1")

		// then
		require.EqualError(t, err, "invalid code: the event is full")
		require.Nil(t, event)
	})

	t.Run("when event not open yet", func(t *testing.T) {
		// given
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event1",
			testsocialevent.WithStartTime(time.Now().Add(time.Hour)))
		nsdClient := namespaced.NewClient(commontest.NewFakeClient(t, event), commontest.HostOperatorNs)

		// when
		event, err := GetAndValidateSocialEvent(ctx, nsdClient, "event1")

		// then
		require.EqualError(t, err, "invalid code: the provided code is not valid yet")
		require.Nil(t, event)
	})

	t.Run("when event already closed", func(t *testing.T) {
		// given
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event1",
			testsocialevent.WithEndTime(time.Now().Add(-time.Hour)))
		nsdClient := namespaced.NewClient(commontest.NewFakeClient(t, event), commontest.HostOperatorNs)

		// when
		event, err := GetAndValidateSocialEvent(ctx, nsdClient, "event1")

		// then
		require.EqualError(t, err, "invalid code: the provided code has expired")
		require.Nil(t, event)
	})

	t.Run("when event does not exist", func(t *testing.T) {
		// given
		event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event1")
		nsdClient := namespaced.NewClient(commontest.NewFakeClient(t, event), commontest.HostOperatorNs)

		// when
		event, err := GetAndValidateSocialEvent(ctx, nsdClient, "unknown")

		// then
		require.EqualError(t, err, "invalid code: the provided code is invalid")
		require.Nil(t, event)
	})
}

func TestUpdateUserSignupWithSocialEvent(t *testing.T) {
	tests := map[string]struct {
		eventOptions            []testsocialevent.Option
		signupOptions           []usersignup.Modifier
		expTargetCluster        string
		expVerificationRequired bool
		expManuallyApproved     bool
	}{
		"nothing set": {
			expManuallyApproved: true,
		},
		"reactivated": {
			signupOptions:       []usersignup.Modifier{usersignup.Deactivated()},
			expManuallyApproved: true,
		},
		"target cluster set": {
			eventOptions:        []testsocialevent.Option{testsocialevent.WithTargetCluster("member")},
			expTargetCluster:    "member",
			expManuallyApproved: true,
		},
		"target cluster overridden": {
			eventOptions:        []testsocialevent.Option{testsocialevent.WithTargetCluster("member1")},
			signupOptions:       []usersignup.Modifier{usersignup.WithTargetCluster("member2")},
			expTargetCluster:    "member1",
			expManuallyApproved: true,
		},
		"verification required when already required": {
			eventOptions: []testsocialevent.Option{
				testsocialevent.WithTargetCluster("member"),
				func(event *toolchainv1alpha1.SocialEvent) {
					event.Spec.VerificationRequired = true
				}},
			signupOptions:           []usersignup.Modifier{usersignup.VerificationRequired()},
			expTargetCluster:        "member",
			expVerificationRequired: true,
		},
		"verification not required when already not required": {
			eventOptions: []testsocialevent.Option{
				func(event *toolchainv1alpha1.SocialEvent) {
					event.Spec.VerificationRequired = true
				}},
			expVerificationRequired: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// given
			event := testsocialevent.NewSocialEvent(commontest.HostOperatorNs, "event1", tc.eventOptions...)
			signup := usersignup.NewUserSignup(tc.signupOptions...)

			// when
			UpdateUserSignupWithSocialEvent(event, signup)

			// then
			assert.Equal(t, "event1", signup.Labels[toolchainv1alpha1.SocialEventUserSignupLabelKey])
			assert.Equal(t, tc.expTargetCluster, signup.Spec.TargetCluster)
			assert.Equal(t, tc.expVerificationRequired, states.VerificationRequired(signup))
			assert.Equal(t, tc.expManuallyApproved, states.ApprovedManually(signup))
			assert.False(t, states.Deactivating(signup))
			assert.False(t, states.Deactivated(signup))
		})
	}
}
