package service

import (
	gocontext "context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/verification/captcha"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	signupcommon "github.com/codeready-toolchain/toolchain-common/pkg/usersignup"
	"github.com/gin-gonic/gin"
	errs "github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// NoSpaceKey is the query key for specifying whether the UserSignup should be created without a Space
	NoSpaceKey = "no-space"
)

var annotationsToRetain = []string{
	toolchainv1alpha1.UserSignupActivationCounterAnnotationKey,
	toolchainv1alpha1.UserSignupLastTargetClusterAnnotationKey,
}

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct { // nolint:revive
	namespaced.Client
	CaptchaChecker captcha.Assessor
}

type SignupServiceOption func(svc *ServiceImpl)

// NewSignupService creates a service object for performing user signup-related activities.
func NewSignupService(client namespaced.Client) *ServiceImpl {
	return &ServiceImpl{
		CaptchaChecker: captcha.Helper{},
		Client:         client,
	}
}

// newUserSignup generates a new UserSignup resource with the specified username and available claims.
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
		log.Info(ctx, fmt.Sprintf("A crtadmin user '%s' just tried to signup", ctx.GetString(context.UsernameKey)))
		return nil, apierrors.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("failed to create usersignup for %s", username))
	}

	userEmail := ctx.GetString(context.EmailKey)
	emailHash := hash.EncodeString(userEmail)

	// Query BannedUsers to check the user has not been banned
	bannedUsers := &toolchainv1alpha1.BannedUserList{}
	if err := s.List(ctx, bannedUsers, client.InNamespace(s.Namespace),
		client.MatchingLabels{toolchainv1alpha1.BannedUserEmailHashLabelKey: hash.EncodeString(userEmail)}); err != nil {
		return nil, err
	}

	for _, bu := range bannedUsers.Items {
		// If the user has been banned, return an error
		if bu.Spec.Email == userEmail {
			return nil, apierrors.NewForbidden(schema.GroupResource{}, "",
				errs.New("The account has been banned due to detected abusive activity or suspicious indicators."))
		}
	}

	verificationRequired, captchaScore, assessmentID := IsPhoneVerificationRequired(s.CaptchaChecker, ctx)
	requestReceivedTime, ok := ctx.Get(context.RequestReceivedTime)
	if !ok {
		requestReceivedTime = time.Now()
	}
	userSignup := &toolchainv1alpha1.UserSignup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      signupcommon.EncodeUserIdentifier(ctx.GetString(context.UsernameKey)),
			Namespace: configuration.Namespace(),
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey: "0",
				toolchainv1alpha1.UserSignupRequestReceivedTimeAnnotationKey: requestReceivedTime.(time.Time).Format(time.RFC3339),
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

// Signup reactivates the deactivated UserSignup resource or creates a new one with the specified username
// if doesn't exist yet.
func (s *ServiceImpl) Signup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	username := ctx.GetString(context.UsernameKey)
	encodedUsername := signupcommon.EncodeUserIdentifier(username)

	// Retrieve UserSignup resource from the host cluster
	userSignup := &toolchainv1alpha1.UserSignup{}
	if err := s.Get(ctx, s.NamespacedName(encodedUsername), userSignup); err != nil {
		if apierrors.IsNotFound(err) {
			// New Signup
			log.WithValues(map[string]interface{}{"encoded_username": encodedUsername}).Info(ctx, "user not found, creating a new one")
			return s.createUserSignup(ctx)
		}
		return nil, err
	}

	// Check UserSignup status to determine whether user signup is deactivated
	signupCondition, found := condition.FindConditionByType(userSignup.Status.Conditions, toolchainv1alpha1.UserSignupComplete)
	if found && signupCondition.Status == apiv1.ConditionTrue && signupCondition.Reason == toolchainv1alpha1.UserSignupUserDeactivatedReason {
		// Signup is deactivated. We need to reactivate it
		return s.reactivateUserSignup(ctx, userSignup)
	}

	return nil, apierrors.NewConflict(schema.GroupResource{}, "", fmt.Errorf(
		"UserSignup [username: %s]. Unable to create UserSignup because there is already an active UserSignup with such a username", username))
}

// createUserSignup creates a new UserSignup resource with the specified username
func (s *ServiceImpl) createUserSignup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	userSignup, err := s.newUserSignup(ctx)
	if err != nil {
		return nil, err
	}

	return userSignup, s.Create(ctx, userSignup)
}

// reactivateUserSignup reactivates the deactivated UserSignup resource with the specified username
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

	return existing, s.Update(ctx, existing)
}

// GetSignup returns Signup resource which represents the corresponding K8s UserSignup
// and MasterUserRecord resources in the host cluster.
// The checkUserSignupCompleted was introduced in order to avoid checking the readiness of the complete condition on the UserSignup in certain situations,
// such as proxy calls for example.
// Returns nil, nil if the UserSignup resource is not found or if it's deactivated.
func (s *ServiceImpl) GetSignup(ctx *gin.Context, username string, checkUserSignupCompleted bool) (*signup.Signup, error) {
	return s.DoGetSignup(ctx, s.Client, username, checkUserSignupCompleted)
}

func (s *ServiceImpl) DoGetSignup(ctx *gin.Context, cl namespaced.Client, username string, checkUserSignupCompleted bool) (*signup.Signup, error) {
	var userSignup *toolchainv1alpha1.UserSignup

	err := signup.PollUpdateSignup(ctx, func() error {
		// Retrieve UserSignup resource from the host cluster
		us := &toolchainv1alpha1.UserSignup{}
		if err := cl.Get(gocontext.TODO(), cl.NamespacedName(signupcommon.EncodeUserIdentifier(username)), us); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		userSignup = us

		// Otherwise if the returned userSignup is nil, return here also
		if userSignup == nil || ctx == nil {
			return nil
		}

		updated := s.auditUserSignupAgainstClaims(ctx, userSignup)

		// If there is no need to update the UserSignup then break out of the loop here (by returning nil)
		// otherwise update the UserSignup
		if updated {
			if err := s.Update(gocontext.TODO(), userSignup); err != nil {
				return err
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
		// UserSignup is banned, let's return a forbidden error
		return nil, apierrors.NewForbidden(schema.GroupResource{}, "",
			errs.New("The account has been banned due to detected abusive activity or suspicious indicators."))
	}

	if !userSignup.Status.ScheduledDeactivationTimestamp.IsZero() {
		signupResponse.EndDate = userSignup.Status.ScheduledDeactivationTimestamp.UTC().Format(time.RFC3339)
	}

	// If UserSignup status is complete as active
	// Retrieve MasterUserRecord resource from the host cluster and use its status
	mur := &toolchainv1alpha1.MasterUserRecord{}
	if err := cl.Get(ctx, cl.NamespacedName(userSignup.Status.CompliantUsername), mur); err != nil {
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
		signupResponse.StartDate = mur.Status.ProvisionedTime.UTC().Format(time.RFC3339)
	}

	memberCluster, defaultNamespace := GetDefaultUserTarget(cl, userSignup.Status.HomeSpace, mur.Name)
	if memberCluster != "" {
		// Retrieve cluster-specific URLs from the status of the corresponding member cluster
		status := &toolchainv1alpha1.ToolchainStatus{}

		if err := cl.Get(ctx, cl.NamespacedName("toolchain-status"), status); err != nil {
			return nil, errs.Wrapf(err, "error when retrieving ToolchainStatus to set Che Dashboard for completed UserSignup %s", userSignup.GetName())
		}
		signupResponse.ProxyURL = status.Status.HostRoutes.ProxyURL
		for _, member := range status.Status.Members {
			if member.ClusterName == memberCluster {
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
		signupResponse.DefaultUserNamespace = defaultNamespace
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

// GetDefaultUserTarget retrieves the target cluster and the default namespace from the Space a user has access to.
// If no spaceName is provided (assuming that this is the home space the target information should be taken from)
// then the logic lists all Spaces user has access to and picks the first one.
// returned values are:
//  1. name of the member cluster the Space is provisioned to
//  2. the name of the default namespace
//
// If the user doesn't have access to any Space, then empty strings are returned
func GetDefaultUserTarget(cl namespaced.Client, spaceName, murName string) (string, string) {
	if spaceName == "" {
		sbs := &toolchainv1alpha1.SpaceBindingList{}
		if err := cl.List(gocontext.TODO(), sbs, client.InNamespace(cl.Namespace),
			client.MatchingLabels{toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey: murName}); err != nil {
			log.Errorf(nil, err, "unable to list spacebindings for MUR %s", murName)
			return "", ""
		}
		if len(sbs.Items) == 0 {
			return "", ""
		}
		spaceNames := make([]string, len(sbs.Items))
		for i, sb := range sbs.Items {
			spaceNames[i] = sb.Spec.Space
		}
		sort.Strings(spaceNames)
		spaceName = spaceNames[0]

	}
	space := &toolchainv1alpha1.Space{}
	if err := cl.Get(gocontext.TODO(), cl.NamespacedName(spaceName), space); err != nil {
		// log error and continue so that the api behaves in a best effort manner
		// ie. if a space isn't listed something went wrong but we still want to return the other spaces if possible
		log.Errorf(nil, err, "unable to get space '%s'", spaceName)
		return "", ""
	}
	var defaultNamespace string

	for _, ns := range space.Status.ProvisionedNamespaces {
		if ns.Type == toolchainv1alpha1.NamespaceTypeDefault {
			// use this namespace if it is the first one found
			defaultNamespace = ns.Name
			break
		}
	}

	return space.Status.TargetCluster, defaultNamespace
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
