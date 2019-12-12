package signup

import (
	"fmt"
	"strconv"

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
	// For example, johnsmith@redhat.com -> johnsmith-at-redhat-com
	CompliantUsername string `json:"compliantUsername"`
	// Original username from the Identity Provider
	Username string `json:"username"`
	Status   Status `json:"status,omitempty"`
}

// Status represents UserSignup resource status
type Status struct {
	// If true then the corresponding user's account is ready to be used
	Ready bool `json:"ready"`
	// Brief reason for the status last transition.
	Reason string `json:"reason"`
	// Human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// ServiceConfiguration represents the config used for the signup service.
type ServiceConfiguration interface {
	GetNamespace() string
}

// Service represents the signup service for controllers.
type Service interface {
	GetSignup(userID string) (*Signup, error)
	CreateUserSignup(username, userID, email string) (*crtapi.UserSignup, error)
}

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct {
	Namespace         string
	UserSignups       kubeclient.UserSignupInterface
	MasterUserRecords kubeclient.MasterUserRecordInterface
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
	}
	return s, nil
}

// CreateUserSignup creates a new UserSignup resource with the specified username and userID
func (s *ServiceImpl) CreateUserSignup(username, userID, userEmail string) (*crtapi.UserSignup, error) {
	userSignup := &crtapi.UserSignup{
		ObjectMeta: v1.ObjectMeta{
			Name:      userID,
			Namespace: s.Namespace,
			Annotations: map[string]string{
				crtapi.UserSignupUserEmailAnnotationKey: userEmail,
			},
		},
		Spec: crtapi.UserSignupSpec{
			TargetCluster: "",
			Approved:      false,
			Username:      username,
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
