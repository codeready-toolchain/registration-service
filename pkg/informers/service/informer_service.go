package service

import (
	"fmt"
	"strconv"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	signupsvc "github.com/codeready-toolchain/registration-service/pkg/signup/service"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	errs "github.com/pkg/errors"

	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const toolchainStatusName = "toolchain-status"

type Option func(f *ServiceImpl)

// ServiceImpl represents the implementation of the informer service.
type ServiceImpl struct { // nolint:revive
	base.BaseService
	informer informers.Informer
}

// NewInformerService creates a service object for getting resources via shared informers
func NewInformerService(context servicecontext.ServiceContext, options ...Option) service.InformerService {
	si := &ServiceImpl{
		BaseService: base.NewBaseService(context),
		informer:    context.Informer(),
	}
	for _, o := range options {
		o(si)
	}
	return si
}

func (s *ServiceImpl) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	obj, err := s.informer.Masteruserrecord.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	mur := &toolchainv1alpha1.MasterUserRecord{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), mur); err != nil {
		log.Errorf(nil, err, "failed to get MasterUserRecord '%s'", name)
		return nil, err
	}
	return mur, err
}

func (s *ServiceImpl) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	obj, err := s.informer.ToolchainStatus.ByNamespace(configuration.Namespace()).Get(toolchainStatusName)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	stat := &toolchainv1alpha1.ToolchainStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), stat); err != nil {
		log.Errorf(nil, err, "failed to get ToolchainStatus %s", toolchainStatusName)
		return nil, err
	}
	return stat, err
}

func (s *ServiceImpl) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	obj, err := s.informer.UserSignup.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	us := &toolchainv1alpha1.UserSignup{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), us); err != nil {
		log.Errorf(nil, err, "failed to get UserSignup '%s'", name)
		return nil, err
	}
	return us, err
}

// GetUserSignupFromIdentifier is used to return the UserSignup resource given a username or user ID
func (s *ServiceImpl) GetUserSignupFromIdentifier(userID, username string) (*toolchainv1alpha1.UserSignup, error) {
	// Retrieve UserSignup resource from the host cluster
	userSignup, err := s.GetUserSignup(signupsvc.EncodeUserIdentifier(username))
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Capture any error here in a separate var, as we need to preserve the original
			userSignup, err2 := s.GetUserSignup(signupsvc.EncodeUserIdentifier(userID))
			if err2 != nil {
				if apierrors.IsNotFound(err2) {
					return nil, err
				}
				return nil, err2
			}
			return userSignup, nil
		}
		return nil, err
	}

	return userSignup, nil
}

// GetSignup duplicates the logic of the 'GetSignup' function in the signup service, except it uses informers to get resources.
// This function can be move to the signup service and replace the GetSignup function there once it is determined to be stable.
func (s *ServiceImpl) GetSignup(userID, username string) (*signup.Signup, error) {
	// Retrieve UserSignup resource from the host cluster, using the specified UserID and username
	userSignup, err := s.GetUserSignupFromIdentifier(userID, username)
	// If an error was returned, then return here
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	// Otherwise if the returned userSignup is nil, return here also
	if userSignup == nil {
		return nil, nil
	}

	signupResponse := &signup.Signup{
		Username: userSignup.Spec.Username,
	}
	if userSignup.Status.CompliantUsername != "" {
		signupResponse.CompliantUsername = userSignup.Status.CompliantUsername
	}

	// Check UserSignup status to determine whether user signup is complete
	approvedCondition, approvedFound := condition.FindConditionByType(userSignup.Status.Conditions, toolchainv1alpha1.UserSignupApproved)
	completeCondition, completeFound := condition.FindConditionByType(userSignup.Status.Conditions, toolchainv1alpha1.UserSignupComplete)
	if !approvedFound || !completeFound || approvedCondition.Status != apiv1.ConditionTrue {
		signupResponse.Status = signup.Status{
			Reason:               toolchainv1alpha1.UserSignupPendingApprovalReason,
			VerificationRequired: states.VerificationRequired(userSignup),
		}
		return signupResponse, nil
	}

	if completeCondition.Status != apiv1.ConditionTrue {
		// UserSignup is not complete
		signupResponse.Status = signup.Status{
			Reason:               completeCondition.Reason,
			Message:              completeCondition.Message,
			VerificationRequired: states.VerificationRequired(userSignup),
		}
		return signupResponse, nil
	} else if completeCondition.Reason == toolchainv1alpha1.UserSignupUserDeactivatedReason {
		// UserSignup is deactivated. Treat it as non-existent.
		return nil, nil
	}

	// If UserSignup status is complete as active
	// Retrieve MasterUserRecord resource from the host cluster and use its status
	mur, err := s.GetMasterUserRecord(userSignup.Status.CompliantUsername)
	if err != nil {
		return nil, errs.Wrap(err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s", userSignup.GetName()))
	}
	murCondition, _ := condition.FindConditionByType(mur.Status.Conditions, toolchainv1alpha1.ConditionReady)
	ready, err := strconv.ParseBool(string(murCondition.Status))
	if err != nil {
		return nil, errs.Wrapf(err, "unable to parse readiness status as bool: %s", murCondition.Status)
	}
	signupResponse.Status = signup.Status{
		Ready:                ready,
		Reason:               murCondition.Reason,
		Message:              murCondition.Message,
		VerificationRequired: states.VerificationRequired(userSignup),
	}
	if mur.Status.UserAccounts != nil && len(mur.Status.UserAccounts) > 0 {
		// Retrieve Console and Che dashboard URLs from the status of the corresponding member cluster
		status, err := s.GetToolchainStatus()
		if err != nil {
			return nil, errs.Wrapf(err, "error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s", userSignup.GetName())
		}
		signupResponse.ProxyURL = status.Status.HostRoutes.ProxyURL
		for _, member := range status.Status.Members {
			if member.ClusterName == mur.Status.UserAccounts[0].Cluster.Name {
				signupResponse.ConsoleURL = member.MemberStatus.Routes.ConsoleURL
				signupResponse.CheDashboardURL = member.MemberStatus.Routes.CheDashboardURL
				signupResponse.APIEndpoint = member.APIEndpoint
				signupResponse.ClusterName = member.ClusterName
				break
			}
		}
	}

	return signupResponse, nil
}
