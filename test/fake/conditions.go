package fake

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
)

func Deactivated() []toolchainv1alpha1.Condition {
	return []toolchainv1alpha1.Condition{
		{
			Type:   toolchainv1alpha1.UserSignupComplete,
			Status: apiv1.ConditionTrue,
			Reason: toolchainv1alpha1.UserSignupUserDeactivatedReason,
		},
		{
			Type:   toolchainv1alpha1.UserSignupApproved,
			Status: apiv1.ConditionFalse,
			Reason: toolchainv1alpha1.UserSignupUserDeactivatedReason,
		},
	}
}
