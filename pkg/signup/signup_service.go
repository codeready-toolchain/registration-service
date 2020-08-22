package signup

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/codeready-toolchain/registration-service/pkg/context"

	"github.com/gin-gonic/gin"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"

	errors2 "github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	PendingApprovalReason = "PendingApproval"
)

// Signup represents Signup resource which is a wrapper of K8s UserSignup
// and the corresponding MasterUserRecord resources.
type Signup struct {
	// The Web Console URL of the cluster which the user was provisioned to
	ConsoleURL string `json:"consoleURL,omitempty"`
	// The Che Dashboard URL of the cluster which the user was provisioned to
	CheDashboardURL string `json:"cheDashboardURL,omitempty"`
	// The complaint username.  This may differ from the corresponding Identity Provider username, because of the the
	// limited character set available for naming (see RFC1123) in K8s. If the username contains characters which are
	// disqualified from the resource name, the username is transformed into an acceptable resource name instead.
	// For example, johnsmith@redhat.com -> johnsmith
	CompliantUsername string `json:"compliantUsername"`
	// Original username from the Identity Provider
	Username string `json:"username"`
	// GivenName from the Identity Provider
	GivenName string `json:"givenName"`
	// FamilyName from the Identity Provider
	FamilyName string `json:"familyName"`
	// Company from the Identity Provider
	Company string `json:"company"`
	Status  Status `json:"status,omitempty"`
}

// Status represents UserSignup resource status
type Status struct {
	// If true then the corresponding user's account is ready to be used
	Ready bool `json:"ready"`
	// Brief reason for the status last transition.
	Reason string `json:"reason"`
	// Human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
	// VerificationRequired is used to determine if a user requires phone verification.
	// The user should not be provisioned if VerificationRequired is set to true.
	// VerificationRequired is set to false when the user is ether exempt from phone verification or has already successfully passed the verification.
	// Default value is false.
	VerificationRequired bool `json:verificationRequired`
}

// ServiceConfiguration represents the config used for the signup service.
type ServiceConfiguration interface {
	GetNamespace() string
}

// Service represents the signup service for controllers.
type Service interface {
	GetSignup(userID string) (*Signup, error)
	CreateUserSignup(ctx *gin.Context) (*crtapi.UserSignup, error)
	GetUserSignup(userID string) (*crtapi.UserSignup, error)
	UpdateUserSignup(userSignup *crtapi.UserSignup) error
}

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct {
	Namespace         string
	UserSignups       kubeclient.UserSignupInterface
	MasterUserRecords kubeclient.MasterUserRecordInterface
	BannedUsers       kubeclient.BannedUserInterface
}

// NewSignupService creates a service object for performing user signup-related activities.
func NewSignupService(cfg ServiceConfiguration) (Service, error) {

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubeclient.NewCRTV1Alpha1Client(k8sConfig, cfg.GetNamespace())
	if err != nil {
		return nil, err
	}

	s := &ServiceImpl{
		Namespace:         cfg.GetNamespace(),
		UserSignups:       client.UserSignups(),
		MasterUserRecords: client.MasterUserRecords(),
		BannedUsers:       client.BannedUsers(),
	}
	return s, nil
}

// CreateUserSignup creates a new UserSignup resource with the specified username and userID
func (s *ServiceImpl) CreateUserSignup(ctx *gin.Context) (*crtapi.UserSignup, error) {
	userEmail := ctx.GetString(context.EmailKey)
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(userEmail))
	emailHash := hex.EncodeToString(md5hash.Sum(nil))

	// Query BannedUsers to check the user has not been banned
	bannedUsers, err := s.BannedUsers.List(userEmail)
	if err != nil {
		return nil, err
	}

	for _, bu := range bannedUsers.Items {
		// If the user has been banned, return an error
		if bu.Spec.Email == userEmail {
			return nil, errors.NewBadRequest("user has been banned")
		}
	}

	userSignup := &crtapi.UserSignup{
		ObjectMeta: v1.ObjectMeta{
			Name:      ctx.GetString(context.SubKey),
			Namespace: s.Namespace,
			Annotations: map[string]string{
				crtapi.UserSignupUserEmailAnnotationKey: userEmail,
			},
			Labels: map[string]string{
				crtapi.UserSignupUserEmailHashLabelKey: emailHash,
			},
		},
		Spec: crtapi.UserSignupSpec{
			TargetCluster: "",
			Approved:      false,
			Username:      ctx.GetString(context.UsernameKey),
			GivenName:     ctx.GetString(context.GivenNameKey),
			FamilyName:    ctx.GetString(context.FamilyNameKey),
			Company:       ctx.GetString(context.CompanyKey),
		},
	}

	created, err := s.UserSignups.Create(userSignup)
	if err != nil {
		return nil, err
	}

	return created, nil
}

// GetSignup returns Signup resource which represents the corresponding K8s UserSignup
// and MasterUserRecord resources in the host cluster.
// Returns nil, nil if the UserSignup resource is not found.
func (s *ServiceImpl) GetSignup(userID string) (*Signup, error) {

	// Retrieve UserSignup resource from the host cluster
	userSignup, err := s.UserSignups.Get(userID)
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
	signupCondition, found := condition.FindConditionByType(userSignup.Status.Conditions, crtapi.UserSignupComplete)
	if !found {
		signupResponse.Status = Status{
			Reason: PendingApprovalReason,
		}
		return signupResponse, nil
	} else if signupCondition.Status != apiv1.ConditionTrue {
		signupResponse.Status = Status{
			Reason:  signupCondition.Reason,
			Message: signupCondition.Message,
		}
		return signupResponse, nil
	}

	// If UserSignup status is complete, retrieve MasterUserRecord resource from the host cluster and use its status
	mur, err := s.MasterUserRecords.Get(userSignup.Status.CompliantUsername)
	if err != nil {
		return nil, errors2.Wrap(err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s", userSignup.GetName()))
	}
	murCondition, _ := condition.FindConditionByType(mur.Status.Conditions, crtapi.ConditionReady)
	ready, err := strconv.ParseBool(string(murCondition.Status))
	if err != nil {
		return nil, errors2.Wrapf(err, "unable to parse readiness status as bool: %s", murCondition.Status)
	}
	signupResponse.Status = Status{
		Ready:   ready,
		Reason:  murCondition.Reason,
		Message: murCondition.Message,
	}
	if mur.Status.UserAccounts != nil && len(mur.Status.UserAccounts) > 0 {
		// TODO Set ConsoleURL in UserSignup.Status. For now it's OK to get it from the first embedded UserAccount status from MUR.
		signupResponse.ConsoleURL = mur.Status.UserAccounts[0].Cluster.ConsoleURL
		signupResponse.CheDashboardURL = mur.Status.UserAccounts[0].Cluster.CheDashboardURL
	}

	return signupResponse, nil
}

func (s *ServiceImpl) GetUserSignup(userID string) (*crtapi.UserSignup, error) {
	// Retrieve UserSignup resource from the host cluster
	userSignup, err := s.UserSignups.Get(userID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return userSignup, nil
}

func (s *ServiceImpl) UpdateUserSignup(userSignup *crtapi.UserSignup) error {
	err := s.UserSignups.Update(userSignup)
	if err != nil {
		return err
	}

	return nil
}
