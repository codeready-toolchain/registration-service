package service

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	errors3 "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"

	"github.com/gin-gonic/gin"
	errors2 "github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceConfiguration represents the config used for the signup service.
type ServiceConfiguration interface {
	GetNamespace() string
	GetVerificationEnabled() bool
	GetVerificationExcludedEmailDomains() []string
	GetVerificationCodeExpiresInMin() int
}

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct {
	base.BaseService
	Config ServiceConfiguration
}

// NewSignupService creates a service object for performing user signup-related activities.
func NewSignupService(context servicecontext.ServiceContext, cfg ServiceConfiguration) service.SignupService {
	return &ServiceImpl{
		BaseService: base.NewBaseService(context),
		Config:      cfg,
	}
}

// newUserSignup generates a new UserSignup resource with the specified username and userID.
// This resource then can be used to create a new UserSignup in the host cluster or to update the existing one.
func (s *ServiceImpl) newUserSignup(ctx *gin.Context) (*v1alpha1.UserSignup, error) {
	userEmail := ctx.GetString(context.EmailKey)
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(userEmail))
	emailHash := hex.EncodeToString(md5hash.Sum(nil))

	// Query BannedUsers to check the user has not been banned
	bannedUsers, err := s.CRTClient().V1Alpha1().BannedUsers().ListByEmail(userEmail)
	if err != nil {
		return nil, err
	}

	for _, bu := range bannedUsers.Items {
		// If the user has been banned, return an error
		if bu.Spec.Email == userEmail {
			return nil, errors.NewBadRequest("user has been banned")
		}
	}

	verificationRequired := s.Config.GetVerificationEnabled()

	// Check if the user's email address is in the list of domains excluded for phone verification
	emailHost := extractEmailHost(userEmail)
	for _, d := range s.Config.GetVerificationExcludedEmailDomains() {
		if strings.EqualFold(d, emailHost) {
			verificationRequired = false
			break
		}
	}

	userSignup := &v1alpha1.UserSignup{
		ObjectMeta: v1.ObjectMeta{
			Name:      ctx.GetString(context.SubKey),
			Namespace: s.Config.GetNamespace(),
			Annotations: map[string]string{
				v1alpha1.UserSignupUserEmailAnnotationKey:           userEmail,
				v1alpha1.UserSignupVerificationCounterAnnotationKey: "0",
			},
			Labels: map[string]string{
				v1alpha1.UserSignupUserEmailHashLabelKey: emailHash,
			},
		},
		Spec: v1alpha1.UserSignupSpec{
			TargetCluster:        "",
			Approved:             false,
			Username:             ctx.GetString(context.UsernameKey),
			GivenName:            ctx.GetString(context.GivenNameKey),
			FamilyName:           ctx.GetString(context.FamilyNameKey),
			Company:              ctx.GetString(context.CompanyKey),
			VerificationRequired: verificationRequired,
		},
	}

	return userSignup, nil
}

func extractEmailHost(email string) string {
	i := strings.LastIndexByte(email, '@')
	return email[i+1:]
}

// Signup reactivates the deactivated UserSignup resource or creates a new one with the specified username and userID
// if doesn't exist yet.
func (s *ServiceImpl) Signup(ctx *gin.Context) (*v1alpha1.UserSignup, error) {
	userID := ctx.GetString(context.SubKey)
	// Retrieve UserSignup resource from the host cluster
	userSignup, err := s.CRTClient().V1Alpha1().UserSignups().Get(userID)
	if err != nil {
		if errors.IsNotFound(err) {
			// New Signup
			return s.createUserSignup(ctx)
		}
		return nil, err
	}

	// Check UserSignup status to determine whether user signup is deactivated
	signupCondition, found := condition.FindConditionByType(userSignup.Status.Conditions, v1alpha1.UserSignupComplete)
	if found && signupCondition.Status == apiv1.ConditionTrue && signupCondition.Reason == v1alpha1.UserSignupUserDeactivatedReason {
		// Signup is deactivated. We need to reactivate it
		return s.reactivateUserSignup(ctx)
	}

	username := ctx.GetString(context.UsernameKey)
	return nil, errors2.Errorf("unable to create UserSignup [id: %s; username: %s] because there is already an active UserSignup with such ID", userID, username)
}

// createUserSignup creates a new UserSignup resource with the specified username and userID
func (s *ServiceImpl) createUserSignup(ctx *gin.Context) (*v1alpha1.UserSignup, error) {
	userSignup, err := s.newUserSignup(ctx)
	if err != nil {
		return nil, err
	}

	return s.CRTClient().V1Alpha1().UserSignups().Create(userSignup)
}

// reactivateUserSignup reactivates the deactivated UserSignup resource with the specified username and userID
func (s *ServiceImpl) reactivateUserSignup(ctx *gin.Context) (*v1alpha1.UserSignup, error) {
	// Create a new signup and replace the existing one by that new one.
	// We don't want to deal with merging/patching the usersignup resource and just want to make it look like a freshly
	// created usersignup resource.
	userSignup, err := s.newUserSignup(ctx)
	if err != nil {
		return nil, err
	}

	return s.CRTClient().V1Alpha1().UserSignups().Update(userSignup)
}

// GetSignup returns Signup resource which represents the corresponding K8s UserSignup
// and MasterUserRecord resources in the host cluster.
// Returns nil, nil if the UserSignup resource is not found or if it's deactivated.
func (s *ServiceImpl) GetSignup(userID string) (*signup.Signup, error) {

	// Retrieve UserSignup resource from the host cluster
	userSignup, err := s.CRTClient().V1Alpha1().UserSignups().Get(userID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	signupResponse := &signup.Signup{
		Username: userSignup.Spec.Username,
	}
	if userSignup.Status.CompliantUsername != "" {
		signupResponse.CompliantUsername = userSignup.Status.CompliantUsername
	}

	// Check UserSignup status to determine whether user signup is complete
	signupCondition, found := condition.FindConditionByType(userSignup.Status.Conditions, v1alpha1.UserSignupComplete)
	if !found {
		signupResponse.Status = signup.Status{
			Reason:               v1alpha1.UserSignupPendingApprovalReason,
			VerificationRequired: userSignup.Spec.VerificationRequired,
		}
		return signupResponse, nil
	} else {
		if signupCondition.Status != apiv1.ConditionTrue {
			// UserSignup is not complete
			signupResponse.Status = signup.Status{
				Reason:               signupCondition.Reason,
				Message:              signupCondition.Message,
				VerificationRequired: userSignup.Spec.VerificationRequired,
			}
			return signupResponse, nil
		} else if signupCondition.Reason == v1alpha1.UserSignupUserDeactivatedReason {
			// UserSignup is deactivated. Treat it as non-existent.
			return nil, nil
		}
	}

	// If UserSignup status is complete as active
	// Retrieve MasterUserRecord resource from the host cluster and use its status
	mur, err := s.CRTClient().V1Alpha1().MasterUserRecords().Get(userSignup.Status.CompliantUsername)
	if err != nil {
		return nil, errors2.Wrap(err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s", userSignup.GetName()))
	}
	murCondition, _ := condition.FindConditionByType(mur.Status.Conditions, v1alpha1.ConditionReady)
	ready, err := strconv.ParseBool(string(murCondition.Status))
	if err != nil {
		return nil, errors2.Wrapf(err, "unable to parse readiness status as bool: %s", murCondition.Status)
	}
	signupResponse.Status = signup.Status{
		Ready:                ready,
		Reason:               murCondition.Reason,
		Message:              murCondition.Message,
		VerificationRequired: userSignup.Spec.VerificationRequired,
	}
	if mur.Status.UserAccounts != nil && len(mur.Status.UserAccounts) > 0 {
		// Retrieve Console and Che dashboard URLs from the status of the corresponding member cluster
		status, err := s.CRTClient().V1Alpha1().ToolchainStatuses().Get()
		if err != nil {
			return nil, errors2.Wrapf(err, "error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s", userSignup.GetName())
		}
		for _, member := range status.Status.Members {
			if member.ClusterName == mur.Status.UserAccounts[0].Cluster.Name {
				signupResponse.ConsoleURL = member.MemberStatus.Routes.ConsoleURL
				signupResponse.CheDashboardURL = member.MemberStatus.Routes.CheDashboardURL
				break
			}
		}
	}

	return signupResponse, nil
}

// GetUserSignup is used to return the actual UserSignup resource instance, rather than the Signup DTO
func (s *ServiceImpl) GetUserSignup(userID string) (*v1alpha1.UserSignup, error) {
	// Retrieve UserSignup resource from the host cluster
	userSignup, err := s.CRTClient().V1Alpha1().UserSignups().Get(userID)
	if err != nil {
		return nil, err
	}

	return userSignup, nil
}

// UpdateUserSignup is used to update the provided UserSignup resource, and returning the updated resource
func (s *ServiceImpl) UpdateUserSignup(userSignup *v1alpha1.UserSignup) (*v1alpha1.UserSignup, error) {
	userSignup, err := s.CRTClient().V1Alpha1().UserSignups().Update(userSignup)
	if err != nil {
		return nil, err
	}

	return userSignup, nil
}

// PhoneNumberAlreadyInUse checks if the phone number has been banned. If so, return
// an internal server error. If not, check if a signup with a different userID
// exists. If so, return an internal server error. Otherwise, return without error.
func (s *ServiceImpl) PhoneNumberAlreadyInUse(userID, e164PhoneNumber string) error {
	bannedUserList, err := s.CRTClient().V1Alpha1().BannedUsers().ListByPhone(e164PhoneNumber)
	if err != nil {
		return errors3.NewInternalError(err, "failed listing banned users")
	}
	if len(bannedUserList.Items) > 0 {
		return errors3.NewForbiddenError("cannot re-register with phone number", "phone number already in use")
	}

	userSignupList, err := s.CRTClient().V1Alpha1().UserSignups().ListByPhone(e164PhoneNumber)
	if err != nil {
		return errors3.NewInternalError(err, "failed listing userSignups")
	}
	for _, signup := range userSignupList.Items {
		if signup.Name != userID {
			return errors3.NewForbiddenError("cannot re-register with phone number", "phone number already in use")
		}
	}

	return nil
}
