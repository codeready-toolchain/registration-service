package service

import (
	"crypto/md5" // nolint:gosec
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"regexp"
	"strconv"
	"strings"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	"github.com/codeready-toolchain/toolchain-common/pkg/usersignup"

	"github.com/gin-gonic/gin"
	errs "github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	DNS1123NameMaximumLength         = 63
	DNS1123NotAllowedCharacters      = "[^-a-z0-9]"
	DNS1123NotAllowedStartCharacters = "^[^a-z0-9]+"
)

// ServiceConfiguration represents the config used for the signup service.
type ServiceConfiguration interface { // nolint:revive
	GetNamespace() string
	GetVerificationEnabled() bool
	GetVerificationExcludedEmailDomains() []string
	GetVerificationCodeExpiresInMin() int
}

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct { // nolint:revive
	base.BaseService
}

// NewSignupService creates a service object for performing user signup-related activities.
func NewSignupService(context servicecontext.ServiceContext) service.SignupService {
	return &ServiceImpl{
		BaseService: base.NewBaseService(context),
	}
}

// newUserSignup generates a new UserSignup resource with the specified username and userID.
// This resource then can be used to create a new UserSignup in the host cluster or to update the existing one.
func (s *ServiceImpl) newUserSignup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	username := ctx.GetString(context.UsernameKey)

	username = usersignup.TransformUsername(username)
	if strings.HasSuffix(username, "crtadmin") {
		log.Info(ctx, fmt.Sprintf("A crtadmin user '%s' just tried to signup - the UserID is: '%s'", ctx.GetString(context.UsernameKey), ctx.GetString(context.SubKey)))
		return nil, apierrors.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("failed to create usersignup for %s", username))
	}

	userEmail := ctx.GetString(context.EmailKey)
	md5hash := md5.New() // nolint:gosec
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
			return nil, apierrors.NewForbidden(schema.GroupResource{}, "", errs.New("user has been banned"))
		}
	}

	cfg := configuration.GetRegistrationServiceConfig()

	verificationRequired := cfg.Verification().Enabled()

	// Check if the user's email address is in the list of domains excluded for phone verification
	emailHost := extractEmailHost(userEmail)
	for _, d := range cfg.Verification().ExcludedEmailDomains() {
		if strings.EqualFold(d, emailHost) {
			verificationRequired = false
			break
		}
	}

	userSignup := &toolchainv1alpha1.UserSignup{
		ObjectMeta: v1.ObjectMeta{
			Name:      EncodeUserIdentifier(ctx.GetString(context.SubKey)),
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey:           userEmail,
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey: "0",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailHashLabelKey: emailHash,
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			TargetCluster: "",
			Userid:        ctx.GetString(context.SubKey),
			Username:      ctx.GetString(context.UsernameKey),
			GivenName:     ctx.GetString(context.GivenNameKey),
			FamilyName:    ctx.GetString(context.FamilyNameKey),
			Company:       ctx.GetString(context.CompanyKey),
			OriginalSub:   ctx.GetString(context.OriginalSubKey),
		},
	}
	states.SetVerificationRequired(userSignup, verificationRequired)

	return userSignup, nil
}

func extractEmailHost(email string) string {
	i := strings.LastIndexByte(email, '@')
	return email[i+1:]
}

// EncodeUserIdentifier transforms a subject value (the user's UserID) to make it DNS-1123 compliant,
// by removing invalid characters, trimming the length and prefixing with a CRC32 checksum if required.
// ### WARNING ### changing this function will cause breakage, as it is used to lookup existing UserSignup
// resources.  If a change is absolutely required, then all existing UserSignup instances must be migrated
// to the new value
func EncodeUserIdentifier(subject string) string {
	// Convert to lower case
	encoded := strings.ToLower(subject)

	// Remove all invalid characters
	nameNotAllowedChars := regexp.MustCompile(DNS1123NotAllowedCharacters)
	encoded = nameNotAllowedChars.ReplaceAllString(encoded, "")

	// Remove invalid start characters
	nameNotAllowedStartChars := regexp.MustCompile(DNS1123NotAllowedStartCharacters)
	encoded = nameNotAllowedStartChars.ReplaceAllString(encoded, "")

	// Add a checksum prefix if the encoded value is different to the original subject value
	if encoded != subject {
		encoded = fmt.Sprintf("%x-%s", crc32.Checksum([]byte(subject), crc32.IEEETable), encoded)
	}

	// Trim if the length exceeds the maximum
	if len(encoded) > DNS1123NameMaximumLength {
		encoded = encoded[0:DNS1123NameMaximumLength]
	}

	return encoded
}

// Signup reactivates the deactivated UserSignup resource or creates a new one with the specified username and userID
// if doesn't exist yet.
func (s *ServiceImpl) Signup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	encodedUserID := EncodeUserIdentifier(ctx.GetString(context.SubKey))
	var userSignup *toolchainv1alpha1.UserSignup
	var err error
	// Retrieve UserSignup resource from the host cluster
	userSignup, err = s.CRTClient().V1Alpha1().UserSignups().Get(encodedUserID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// The UserSignup could not be located by its encoded UserID, attempt to load it using its encoded Username instead
			encodedUsername := EncodeUserIdentifier(ctx.GetString(context.UsernameKey))
			userSignup, err = s.CRTClient().V1Alpha1().UserSignups().Get(encodedUsername)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// New Signup
					log.WithValues(map[string]interface{}{"encoded_user_id": encodedUserID}).Info(ctx, "user not found, creating a new one")
					return s.createUserSignup(ctx)
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Check UserSignup status to determine whether user signup is deactivated
	signupCondition, found := condition.FindConditionByType(userSignup.Status.Conditions, toolchainv1alpha1.UserSignupComplete)
	if found && signupCondition.Status == apiv1.ConditionTrue && signupCondition.Reason == toolchainv1alpha1.UserSignupUserDeactivatedReason {
		// Signup is deactivated. We need to reactivate it
		return s.reactivateUserSignup(ctx, userSignup)
	}

	username := ctx.GetString(context.UsernameKey)
	return nil, apierrors.NewConflict(schema.GroupResource{}, "", fmt.Errorf("UserSignup [id: %s; username: %s]. Unable to create UserSignup because there is already an active UserSignup with such ID", encodedUserID, username))
}

// createUserSignup creates a new UserSignup resource with the specified username and userID
func (s *ServiceImpl) createUserSignup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	userSignup, err := s.newUserSignup(ctx)
	if err != nil {
		return nil, err
	}

	return s.CRTClient().V1Alpha1().UserSignups().Create(userSignup)
}

// reactivateUserSignup reactivates the deactivated UserSignup resource with the specified username and userID
func (s *ServiceImpl) reactivateUserSignup(ctx *gin.Context, existing *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
	// Update the existing usersignup's spec and annotations/labels by new values from a freshly generated one.
	// We don't want to deal with merging/patching the usersignup resource
	// and just want to reset the spec and annotations/labels so they are the same as in a freshly created usersignup resource.
	newUserSignup, err := s.newUserSignup(ctx)
	if err != nil {
		return nil, err
	}
	log.WithValues(map[string]interface{}{toolchainv1alpha1.UserSignupActivationCounterAnnotationKey: existing.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]}).
		Info(ctx, "reactivating user")

	// (don't override) the `toolchain.dev.openshift.com/activation-counter` if it is already set in the existing UserSignup
	if c, exists := existing.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]; exists {
		newUserSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey] = c
	}
	existing.Annotations = newUserSignup.Annotations
	existing.Labels = newUserSignup.Labels
	existing.Spec = newUserSignup.Spec

	return s.CRTClient().V1Alpha1().UserSignups().Update(existing)
}

// GetSignup returns Signup resource which represents the corresponding K8s UserSignup
// and MasterUserRecord resources in the host cluster.
// Returns nil, nil if the UserSignup resource is not found or if it's deactivated.
func (s *ServiceImpl) GetSignup(userID, username string) (*signup.Signup, error) {
	// Declare variables up front
	var userSignup *toolchainv1alpha1.UserSignup
	var err error

	// Retrieve UserSignup resource from the host cluster, using the specified UserID
	userSignup, err = s.CRTClient().V1Alpha1().UserSignups().Get(EncodeUserIdentifier(userID))
	if err != nil {
		// If the UserSignup was not found, attempt to find it using the specified username instead
		if apierrors.IsNotFound(err) {
			userSignup, err = s.CRTClient().V1Alpha1().UserSignups().Get(EncodeUserIdentifier(username))

			// Otherwise if there was any other error, return it instead
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, err
			}
		} else {
			return nil, err
		}
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
	mur, err := s.CRTClient().V1Alpha1().MasterUserRecords().Get(userSignup.Status.CompliantUsername)
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
		status, err := s.CRTClient().V1Alpha1().ToolchainStatuses().Get()
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

// GetUserSignup is used to return the actual UserSignup resource instance, rather than the Signup DTO
func (s *ServiceImpl) GetUserSignup(userID, username string) (*toolchainv1alpha1.UserSignup, error) {
	// Declare variables up front
	var userSignup *toolchainv1alpha1.UserSignup
	var err error
	var err2 error

	// Retrieve UserSignup resource from the host cluster
	userSignup, err = s.CRTClient().V1Alpha1().UserSignups().Get(EncodeUserIdentifier(userID))
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Capture any error here in a separate var, as we need to preserve the original
			userSignup, err2 = s.CRTClient().V1Alpha1().UserSignups().Get(EncodeUserIdentifier(username))
			if err2 != nil {
				// If the UserSignup cannot be located by its username either, then return the original error
				if apierrors.IsNotFound(err2) {
					return nil, err
				}
				return nil, err
			}
			return userSignup, nil
		}
		return nil, err
	}

	return userSignup, nil
}

// UpdateUserSignup is used to update the provided UserSignup resource, and returning the updated resource
func (s *ServiceImpl) UpdateUserSignup(userSignup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
	userSignup, err := s.CRTClient().V1Alpha1().UserSignups().Update(userSignup)
	if err != nil {
		return nil, err
	}

	return userSignup, nil
}

// PhoneNumberAlreadyInUse checks if the phone number has been banned. If so, return
// an internal server error. If not, check if an active (non-deactivated) UserSignup with a different userID
// and email address exists. If so, return an internal server error. Otherwise, return without error.
// Either the actual phone number, or the md5 hash of the phone number may be provided here.
func (s *ServiceImpl) PhoneNumberAlreadyInUse(userID, phoneNumberOrHash string) error {
	bannedUserList, err := s.CRTClient().V1Alpha1().BannedUsers().ListByPhoneNumberOrHash(phoneNumberOrHash)
	if err != nil {
		return errors.NewInternalError(err, "failed listing banned users")
	}
	if len(bannedUserList.Items) > 0 {
		return errors.NewForbiddenError("cannot re-register with phone number", "phone number already in use")
	}

	userSignupList, err := s.CRTClient().V1Alpha1().UserSignups().ListActiveSignupsByPhoneNumberOrHash(phoneNumberOrHash)
	if err != nil {
		return errors.NewInternalError(err, "failed listing userSignups")
	}
	for _, signup := range userSignupList.Items {
		if signup.Spec.Userid != userID && !states.Deactivated(&signup) { // nolint:gosec
			return errors.NewForbiddenError("cannot re-register with phone number",
				"phone number already in use")
		}
	}

	return nil
}
