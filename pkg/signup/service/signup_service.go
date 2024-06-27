package service

import (
	"fmt"
	"hash/crc32"
	"regexp"
	"strings"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/verification/captcha"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	"github.com/gin-gonic/gin"
	errs "github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	DNS1123NameMaximumLength = 63

	// NoSpaceKey is the query key for specifying whether the UserSignup should be created without a Space
	NoSpaceKey = "no-space"
)

var annotationsToRetain = []string{
	toolchainv1alpha1.UserSignupActivationCounterAnnotationKey,
	toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey,
}

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct { // nolint:revive
	base.BaseService
	defaultProvider ResourceProvider
	CaptchaChecker  captcha.Assessor
}

type SignupServiceOption func(svc *ServiceImpl)

// NewSignupService creates a service object for performing user signup-related activities.
func NewSignupService(context servicecontext.ServiceContext, opts ...SignupServiceOption) service.SignupService {

	s := &ServiceImpl{
		BaseService:     base.NewBaseService(context),
		defaultProvider: crtClientProvider{context.CRTClient()},
		CaptchaChecker:  captcha.Helper{},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// newUserSignup generates a new UserSignup resource with the specified username and userID.
// This resource then can be used to create a new UserSignup in the host cluster or to update the existing one.
func (s *ServiceImpl) newUserSignup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	username := ctx.GetString(context.UsernameKey)

	userID := ctx.GetString(context.UserIDKey)
	accountID := ctx.GetString(context.AccountIDKey)

	if userID == "" || accountID == "" {
		log.Infof(ctx, "Missing essential claims from token - [user_id:%s][account_id:%s] for user [%s], sub [%s]",
			userID, accountID, username, ctx.GetString(context.SubKey))
	}

	if isCRTAdmin(username) {
		log.Info(ctx, fmt.Sprintf("A crtadmin user '%s' just tried to signup - the UserID is: '%s'", ctx.GetString(context.UsernameKey), ctx.GetString(context.SubKey)))
		return nil, apierrors.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("failed to create usersignup for %s", username))
	}

	userEmail := ctx.GetString(context.EmailKey)
	emailHash := hash.EncodeString(userEmail)

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

	verificationRequired, captchaScore, assessmentID := IsPhoneVerificationRequired(s.CaptchaChecker, ctx)

	userSignup := &toolchainv1alpha1.UserSignup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EncodeUserIdentifier(ctx.GetString(context.UsernameKey)),
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey: "0",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailHashLabelKey: emailHash,
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			TargetCluster: "",

			IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
				PropagatedClaims: toolchainv1alpha1.PropagatedClaims{
					Sub:         ctx.GetString(context.SubKey),
					UserID:      ctx.GetString(context.UserIDKey),
					AccountID:   ctx.GetString(context.AccountIDKey),
					OriginalSub: ctx.GetString(context.OriginalSubKey),
					Email:       userEmail,
				},
				PreferredUsername: ctx.GetString(context.UsernameKey),
				GivenName:         ctx.GetString(context.GivenNameKey),
				FamilyName:        ctx.GetString(context.FamilyNameKey),
				Company:           ctx.GetString(context.CompanyKey),
			},
		},
	}

	if captchaScore > -1.0 {
		userSignup.Annotations[toolchainv1alpha1.UserSignupCaptchaScoreAnnotationKey] = fmt.Sprintf("%.1f", captchaScore)
		// store assessment ID as annotation in UserSignup so that captcha assessments can be annotated later on eg. when a user is banned
		userSignup.Annotations[toolchainv1alpha1.UserSignupCaptchaAssessmentIDAnnotationKey] = assessmentID
	}

	states.SetVerificationRequired(userSignup, verificationRequired)

	// set the skip-auto-create-space annotation to true if the no-space query parameter was set to true
	if param, _ := ctx.GetQuery(NoSpaceKey); param == "true" {
		log.Info(ctx, fmt.Sprintf("setting '%s' annotation to true", toolchainv1alpha1.SkipAutoCreateSpaceAnnotationKey))
		userSignup.Annotations[toolchainv1alpha1.SkipAutoCreateSpaceAnnotationKey] = "true"
	}

	return userSignup, nil
}

func isCRTAdmin(username string) bool {
	newUsername := regexp.MustCompile("[^A-Za-z0-9]").ReplaceAllString(strings.Split(username, "@")[0], "-")
	return strings.HasSuffix(newUsername, "crtadmin")
}

/*
IsPhoneVerificationRequired determines whether phone verification is required

Returns true in the following cases:
1. Captcha configuration is disabled
2. The captcha token is invalid
3. Captcha failed with an error or the assessment failed

Returns false in the following cases:
1. Overall verification configuration is disabled
2. User's email domain is excluded
3. Captcha is enabled and the assessment is successful

Returns true/false to dictate whether phone verification is required.
Returns the captcha score if the assessment was successful, otherwise returns -1 which will
prevent the score from being set in the UserSignup annotation.

Returns the assessment ID if a captcha assessment was completed
*/
func IsPhoneVerificationRequired(captchaChecker captcha.Assessor, ctx *gin.Context) (bool, float32, string) {
	cfg := configuration.GetRegistrationServiceConfig()

	// skip verification if verification is disabled
	if !cfg.Verification().Enabled() {
		return false, -1, ""
	}

	// skip verification for excluded email domains
	userEmail := ctx.GetString(context.EmailKey)
	emailHost := extractEmailHost(userEmail)
	for _, d := range cfg.Verification().ExcludedEmailDomains() {
		if strings.EqualFold(d, emailHost) {
			return false, -1, ""
		}
	}

	// require verification if captcha is disabled
	if !cfg.Verification().CaptchaEnabled() {
		return true, -1, ""
	}

	// require verification if context is invalid
	if ctx.Request == nil {
		log.Error(ctx, nil, "no request in context")
		return true, -1, ""
	}

	// require verification if captcha token is invalid
	captchaToken, exists := ctx.Request.Header["Recaptcha-Token"]
	if !exists || len(captchaToken) != 1 {
		log.Error(ctx, nil, "no valid captcha token found in request header")
		return true, -1, ""
	}

	// do captcha assessment

	// require verification if captcha failed
	assessment, err := captchaChecker.CompleteAssessment(ctx, cfg, captchaToken[0])
	if err != nil {
		log.Error(ctx, err, "signup assessment failed")
		return true, -1, ""
	}

	// require verification if captcha score is too low
	score := assessment.GetRiskAnalysis().GetScore()
	threshold := cfg.Verification().CaptchaScoreThreshold()
	if score < threshold {
		log.Info(ctx, fmt.Sprintf("the risk analysis score '%.1f' did not meet the expected threshold '%.1f'", score, threshold))
		return true, score, assessment.GetName()
	}

	// verification not required, score is above threshold
	return false, score, assessment.GetName()
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
	// Sanitize subject to be compliant with DNS labels format (RFC-1123)
	encoded := sanitizeDNS1123(subject)

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

func sanitizeDNS1123(str string) string {
	// convert to lowercase
	lstr := strings.ToLower(str)

	// remove unwanted characters
	b := strings.Builder{}
	for _, r := range lstr {
		switch {
		case r >= '0' && r <= '9':
			fallthrough
		case r >= 'a' && r <= 'z':
			fallthrough
		case r == '-':
			b.WriteRune(r)
		}
	}

	// remove leading and trailing '-'
	return strings.Trim(b.String(), "-")
}

// Signup reactivates the deactivated UserSignup resource or creates a new one with the specified username and userID
// if doesn't exist yet.
func (s *ServiceImpl) Signup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	encodedUserID := EncodeUserIdentifier(ctx.GetString(context.SubKey))

	// Retrieve UserSignup resource from the host cluster
	userSignup, err := s.CRTClient().V1Alpha1().UserSignups().Get(encodedUserID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// The UserSignup could not be located by its encoded UserID, attempt to load it using its encoded PreferredUsername instead
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
	return nil, apierrors.NewConflict(schema.GroupResource{}, "", fmt.Errorf(
		"UserSignup [id: %s; username: %s]. Unable to create UserSignup because there is already an active UserSignup with such ID", encodedUserID, username))
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

	// don't override any of the annotations that need to be retained if they are already set in the existing UserSignup
	for _, a := range annotationsToRetain {
		if c, exists := existing.Annotations[a]; exists {
			newUserSignup.Annotations[a] = c
		}
	}

	existing.Annotations = newUserSignup.Annotations
	existing.Labels = newUserSignup.Labels
	existing.Spec = newUserSignup.Spec

	return s.CRTClient().V1Alpha1().UserSignups().Update(existing)
}

// GetSignup returns Signup resource which represents the corresponding K8s UserSignup
// and MasterUserRecord resources in the host cluster.
// Returns nil, nil if the UserSignup resource is not found or if it's deactivated.
func (s *ServiceImpl) GetSignup(ctx *gin.Context, userID, username string) (*signup.Signup, error) {
	return s.DoGetSignup(ctx, s.defaultProvider, userID, username, true)
}

// GetSignupFromInformer uses the same logic of the 'GetSignup' function, except it uses informers to get resources.
// This function and the ResourceProvider abstraction can replace the original GetSignup function once it is determined to be stable.
// The checkUserSignupCompleted was introduced in order to avoid checking the readiness of the complete condition on the UserSignup in certain situations,
// such as proxy calls for example.
func (s *ServiceImpl) GetSignupFromInformer(ctx *gin.Context, userID, username string, checkUserSignupCompleted bool) (*signup.Signup, error) {
	return s.DoGetSignup(ctx, s.Services().InformerService(), userID, username, checkUserSignupCompleted)
}

func (s *ServiceImpl) DoGetSignup(ctx *gin.Context, provider ResourceProvider, userID, username string, checkUserSignupCompleted bool) (*signup.Signup, error) {
	var userSignup *toolchainv1alpha1.UserSignup
	var err error

	err = signup.PollUpdateSignup(ctx, func() error {
		// Retrieve UserSignup resource from the host cluster, using the specified UserID and username
		var getError error
		userSignup, getError = s.DoGetUserSignupFromIdentifier(provider, userID, username)
		// If an error was returned, then return here
		if getError != nil {
			if apierrors.IsNotFound(getError) {
				return nil
			}
			return getError
		}

		// Otherwise if the returned userSignup is nil, return here also
		if userSignup == nil || ctx == nil {
			return nil
		}

		updated := s.auditUserSignupAgainstClaims(ctx, userSignup)

		// If there is no need to update the UserSignup then break out of the loop here (by returning nil)
		// otherwise update the UserSignup
		if updated {
			var updateErr error
			userSignup, updateErr = s.UpdateUserSignup(userSignup)
			if updateErr != nil {
				return updateErr
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if userSignup == nil {
		return nil, nil
	}

	signupResponse := &signup.Signup{
		Name:     userSignup.GetName(),
		Username: userSignup.Spec.IdentityClaims.PreferredUsername,
	}
	if userSignup.Status.CompliantUsername != "" {
		signupResponse.CompliantUsername = userSignup.Status.CompliantUsername
	}

	// Check UserSignup status to determine whether user signup is complete
	_, approvedFound := condition.FindConditionByType(userSignup.Status.Conditions, toolchainv1alpha1.UserSignupApproved)
	completeCondition, completeFound := condition.FindConditionByType(userSignup.Status.Conditions, toolchainv1alpha1.UserSignupComplete)
	if !approvedFound || !completeFound ||
		condition.IsFalseWithReason(userSignup.Status.Conditions,
			toolchainv1alpha1.UserSignupApproved, toolchainv1alpha1.UserSignupPendingApprovalReason) {
		log.Info(nil, fmt.Sprintf("usersignup: %s is pending approval", userSignup.GetName()))

		signupResponse.Status = signup.Status{
			Reason:               toolchainv1alpha1.UserSignupPendingApprovalReason,
			VerificationRequired: states.VerificationRequired(userSignup),
		}
		return signupResponse, nil
	}

	// in proxy, we don't care if the UserSignup is completed, since sometimes it might be transitioning from complete to provisioning
	// which causes issues with some proxy calls, that's why we introduced the checkUserSignupCompleted parameter.
	// See Jira: https://issues.redhat.com/browse/SANDBOX-375
	if completeCondition.Status != apiv1.ConditionTrue && checkUserSignupCompleted {
		// UserSignup is not complete
		log.Info(nil, fmt.Sprintf("usersignup: %s is not complete", userSignup.GetName()))
		signupResponse.Status = signup.Status{
			Reason:               completeCondition.Reason,
			Message:              completeCondition.Message,
			VerificationRequired: states.VerificationRequired(userSignup),
		}
		return signupResponse, nil
	} else if completeCondition.Reason == toolchainv1alpha1.UserSignupUserDeactivatedReason {
		log.Info(nil, fmt.Sprintf("usersignup: %s is deactivated", userSignup.GetName()))
		// UserSignup is deactivated. Treat it as non-existent.
		return nil, nil
	} else if completeCondition.Reason == toolchainv1alpha1.UserSignupUserBannedReason {
		log.Info(nil, fmt.Sprintf("usersignup: %s is banned", userSignup.GetName()))
		// UserSignup is banned, let's return a pending approval reason to the client.
		signupResponse.Status = signup.Status{
			Reason: toolchainv1alpha1.UserSignupPendingApprovalReason,
		}
		return signupResponse, nil
	}

	if !userSignup.Status.ScheduledDeactivationTimestamp.IsZero() {
		signupResponse.EndDate = userSignup.Status.ScheduledDeactivationTimestamp.Format(time.RFC3339)
	}

	// If UserSignup status is complete as active
	// Retrieve MasterUserRecord resource from the host cluster and use its status
	mur, err := provider.GetMasterUserRecord(userSignup.Status.CompliantUsername)
	if err != nil {
		return nil, errs.Wrap(err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s", userSignup.GetName()))
	}
	murCondition, _ := condition.FindConditionByType(mur.Status.Conditions, toolchainv1alpha1.ConditionReady)
	// the MUR may not be ready immediately, so let's set it to not ready if the Ready condition it's not True,
	// and the client can keep calling back until it's ready.
	ready := murCondition.Status == apiv1.ConditionTrue
	log.Info(nil, fmt.Sprintf("mur ready condition is: %t", ready))
	signupResponse.Status = signup.Status{
		Ready:                ready,
		Reason:               murCondition.Reason,
		Message:              murCondition.Message,
		VerificationRequired: states.VerificationRequired(userSignup),
	}

	if mur.Status.ProvisionedTime != nil {
		signupResponse.StartDate = mur.Status.ProvisionedTime.Format(time.RFC3339)
	}

	if mur.Status.UserAccounts != nil && len(mur.Status.UserAccounts) > 0 {
		// Retrieve cluster-specific URLs from the status of the corresponding member cluster
		status, err := provider.GetToolchainStatus()
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

		// set RHODS member URL
		signupResponse.RHODSMemberURL = getRHODSMemberURL(*signupResponse)

		// set default user namespace
		signupResponse.DefaultUserNamespace = GetDefaultUserNamespace(provider, *signupResponse)
	}

	return signupResponse, nil
}

// auditUserSignupAgainstClaims compares the properties of the specified UserSignup against the claims contained in the
// user's access token and updates the UserSignup if necessary.  If updates were made, the function returns true
// otherwise it returns false.
func (s *ServiceImpl) auditUserSignupAgainstClaims(ctx *gin.Context, userSignup *toolchainv1alpha1.UserSignup) bool {

	updated := false

	updateIfRequired := func(ctx *gin.Context, key, existing string, updated bool) (string, bool) {
		if val, ok := ctx.Get(key); ok && val != nil && len(val.(string)) > 0 && val != existing {
			return val.(string), true
		}
		return existing, updated
	}

	c := userSignup.Spec.IdentityClaims

	// Check each of the properties of IdentityClaimsEmbedded individually
	c.Sub, updated = updateIfRequired(ctx, context.SubKey, c.Sub, updated)
	c.UserID, updated = updateIfRequired(ctx, context.UserIDKey, c.UserID, updated)
	c.AccountID, updated = updateIfRequired(ctx, context.AccountIDKey, c.AccountID, updated)
	c.OriginalSub, updated = updateIfRequired(ctx, context.OriginalSubKey, c.OriginalSub, updated)
	c.Email, updated = updateIfRequired(ctx, context.EmailKey, c.Email, updated)
	c.PreferredUsername, updated = updateIfRequired(ctx, context.UsernameKey, c.PreferredUsername, updated)
	c.GivenName, updated = updateIfRequired(ctx, context.GivenNameKey, c.GivenName, updated)
	c.FamilyName, updated = updateIfRequired(ctx, context.FamilyNameKey, c.FamilyName, updated)
	c.Company, updated = updateIfRequired(ctx, context.CompanyKey, c.Company, updated)

	userSignup.Spec.IdentityClaims = c

	// make sure that labels and annotations are initiated
	if userSignup.Labels == nil {
		userSignup.Labels = map[string]string{}
	}
	if userSignup.Annotations == nil {
		userSignup.Annotations = map[string]string{}
	}

	// make sure that he email hash matches the email
	emailHash := hash.EncodeString(c.Email)
	if userSignup.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey] != emailHash {
		userSignup.Labels[toolchainv1alpha1.UserSignupUserEmailHashLabelKey] = emailHash
		updated = true
	}

	return updated
}

// GetUserSignupFromIdentifier is used to return the actual UserSignup resource instance, rather than the Signup DTO
func (s *ServiceImpl) GetUserSignupFromIdentifier(userID, username string) (*toolchainv1alpha1.UserSignup, error) {
	return s.DoGetUserSignupFromIdentifier(s.defaultProvider, userID, username)
}

// GetUserSignupFromIdentifier is used to return the actual UserSignup resource instance, rather than the Signup DTO
func (s *ServiceImpl) DoGetUserSignupFromIdentifier(provider ResourceProvider, userID, username string) (*toolchainv1alpha1.UserSignup, error) {
	// Retrieve UserSignup resource from the host cluster
	userSignup, err := provider.GetUserSignup(EncodeUserIdentifier(username))
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Capture any error here in a separate var, as we need to preserve the original
			userSignup, err2 := provider.GetUserSignup(EncodeUserIdentifier(userID))
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

// UpdateUserSignup is used to update the provided UserSignup resource, and returning the updated resource
func (s *ServiceImpl) UpdateUserSignup(userSignup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
	userSignup, err := s.CRTClient().V1Alpha1().UserSignups().Update(userSignup)
	if err != nil {
		return nil, err
	}

	return userSignup, nil
}

// PhoneNumberAlreadyInUse checks if the phone number has been banned. If so, return
// an internal server error. If not, check if an active (non-deactivated) UserSignup with a different userID and username
// and email address exists. If so, return an internal server error. Otherwise, return without error.
// Either the actual phone number, or the md5 hash of the phone number may be provided here.
func (s *ServiceImpl) PhoneNumberAlreadyInUse(userID, username, phoneNumberOrHash string) error {
	bannedUserList, err := s.CRTClient().V1Alpha1().BannedUsers().ListByPhoneNumberOrHash(phoneNumberOrHash)
	if err != nil {
		return errors.NewInternalError(err, "failed listing banned users")
	}
	if len(bannedUserList.Items) > 0 {
		return errors.NewForbiddenError("cannot re-register with phone number", "phone number already in use")
	}

	userSignups, err := s.CRTClient().V1Alpha1().UserSignups().ListActiveSignupsByPhoneNumberOrHash(phoneNumberOrHash)
	if err != nil {
		return errors.NewInternalError(err, "failed listing userSignups")
	}
	for _, signup := range userSignups {
		if signup.Spec.IdentityClaims.Sub != userID && signup.Spec.IdentityClaims.PreferredUsername != username && !states.Deactivated(signup) { // nolint:gosec
			return errors.NewForbiddenError("cannot re-register with phone number",
				"phone number already in use")
		}
	}

	return nil
}

func GetDefaultUserNamespace(provider ResourceProvider, signup signup.Signup) string {
	sbSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey, selection.Equals, []string{signup.CompliantUsername})
	if err != nil {
		log.Errorf(nil, err, "unable to create spacebindings selector for signup %s", signup.Name)
		return ""
	}

	requirements := []labels.Requirement{*sbSelector}

	sbs, err := provider.ListSpaceBindings(requirements...)
	if err != nil {
		log.Errorf(nil, err, "unable to list spacebindings for signup %s", signup.Name)
		return ""
	}

	// iterate through the SpaceBindings to find the Spaces that the user has access to, then look for the default namespace with a preference for a Space created by the user
	var defaultNamespace string
	var createdSpaceFound bool
	for _, sb := range sbs {
		spaceName := sb.Labels[toolchainv1alpha1.SpaceBindingSpaceLabelKey]
		if spaceName == "" { // space may not be initialized
			// log error and continue so that the api behaves in a best effort manner
			log.Errorf(nil, fmt.Errorf("spacebinding has no '%s' label", toolchainv1alpha1.SpaceBindingSpaceLabelKey), "unable to get space '%s'", spaceName)
			continue
		}
		space, err := provider.GetSpace(spaceName)
		if err != nil {
			// log error and continue so that the api behaves in a best effort manner
			// ie. if a space isn't listed something went wrong but we still want to return the other spaces if possible
			log.Errorf(nil, err, "unable to get space '%s'", spaceName)
			continue
		}

		createdSpaceFound = space.Labels[toolchainv1alpha1.SpaceCreatorLabelKey] == signup.Name

		for _, ns := range space.Status.ProvisionedNamespaces {
			if ns.Type == toolchainv1alpha1.NamespaceTypeDefault {
				// use this namespace if it is the first one found or if the space was created by the user
				if defaultNamespace == "" || createdSpaceFound {
					defaultNamespace = ns.Name
				}
				break
			}
		}

		// if the space was created by the user and we have found a default namespace then we can stop looking because this is the best case scenario
		if createdSpaceFound && defaultNamespace != "" {
			break
		}
	}

	return defaultNamespace
}

func getRHODSMemberURL(signup signup.Signup) string {
	return getAppsURL("rhods-dashboard-redhat-ods-applications", signup)
}

// getAppsURL returns a URL for the specific app
// for example for the "devspaces" app and api server "https://api.host.openshiftapps.com:6443"
// it will return "https://devspaces.apps.host.openshiftapps.com"
func getAppsURL(appRouteName string, signup signup.Signup) string {
	index := strings.Index(signup.ConsoleURL, ".apps")
	if index == -1 {
		return ""
	}
	// get the appsURL eg. .apps.host.openshiftapps.com
	appsURL := signup.ConsoleURL[index:]
	return fmt.Sprintf("https://%s%s", appRouteName, appsURL)
}
