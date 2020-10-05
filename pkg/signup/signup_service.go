package signup

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"

	errors3 "github.com/codeready-toolchain/registration-service/pkg/errors"

	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
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

	s := &ServiceImpl{
		BaseService: base.NewBaseService(context),
		Config:      cfg,
	}
	return s
}

// CreateUserSignup creates a new UserSignup resource with the specified username and userID
func (s *ServiceImpl) CreateUserSignup(ctx *gin.Context) (*v1alpha1.UserSignup, error) {
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
				v1alpha1.UserSignupApprovedLabelKey:      v1alpha1.UserSignupApprovedLabelValueFalse,
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

	created, err := s.CRTClient().V1Alpha1().UserSignups().Create(userSignup)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func extractEmailHost(email string) string {
	i := strings.LastIndexByte(email, '@')
	return email[i+1:]
}

// GetSignup returns Signup resource which represents the corresponding K8s UserSignup
// and MasterUserRecord resources in the host cluster.
// Returns nil, nil if the UserSignup resource is not found.
func (s *ServiceImpl) GetSignup(userID string) (*Signup, error) {

	// Retrieve UserSignup resource from the host cluster
	userSignup, err := s.CRTClient().V1Alpha1().UserSignups().Get(userID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	signupResponse := &Signup{
		Username: userSignup.Spec.Username,
	}
	if userSignup.Status.CompliantUsername != "" {
		signupResponse.CompliantUsername = userSignup.Status.CompliantUsername
	}

	// Check UserSignup status to determine whether user signup is complete
	signupCondition, found := condition.FindConditionByType(userSignup.Status.Conditions, v1alpha1.UserSignupComplete)
	if !found {
		signupResponse.Status = Status{
			Reason:               v1alpha1.UserSignupPendingApprovalReason,
			VerificationRequired: userSignup.Spec.VerificationRequired,
		}
		return signupResponse, nil
	} else if signupCondition.Status != apiv1.ConditionTrue {
		signupResponse.Status = Status{
			Reason:               signupCondition.Reason,
			Message:              signupCondition.Message,
			VerificationRequired: userSignup.Spec.VerificationRequired,
		}
		return signupResponse, nil
	}

	// If UserSignup status is complete, retrieve MasterUserRecord resource from the host cluster and use its status
	mur, err := s.CRTClient().V1Alpha1().MasterUserRecords().Get(userSignup.Status.CompliantUsername)
	if err != nil {
		return nil, errors2.Wrap(err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s", userSignup.GetName()))
	}
	murCondition, _ := condition.FindConditionByType(mur.Status.Conditions, v1alpha1.ConditionReady)
	ready, err := strconv.ParseBool(string(murCondition.Status))
	if err != nil {
		return nil, errors2.Wrapf(err, "unable to parse readiness status as bool: %s", murCondition.Status)
	}
	signupResponse.Status = Status{
		Ready:                ready,
		Reason:               murCondition.Reason,
		Message:              murCondition.Message,
		VerificationRequired: userSignup.Spec.VerificationRequired,
	}
	if mur.Status.UserAccounts != nil && len(mur.Status.UserAccounts) > 0 {
		// TODO Set ConsoleURL in UserSignup.Status. For now it's OK to get it from the first embedded UserAccount status from MUR.
		signupResponse.ConsoleURL = mur.Status.UserAccounts[0].Cluster.ConsoleURL
		signupResponse.CheDashboardURL = mur.Status.UserAccounts[0].Cluster.CheDashboardURL
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
