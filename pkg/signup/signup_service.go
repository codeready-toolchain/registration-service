package signup

import (
	"fmt"
	"strings"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"

	errors2 "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
)

// Signup represents Signup resource which is a wrapper of K8s UserSignup
// and the corresponding MasterUserRecord resources.
type Signup struct {
	// The cluster in which the user is provisioned in
	// If not set then the target cluster will be picked automatically
	TargetCluster string `json:"targetCluster,omitempty"`
	// The username.  This may differ from the corresponding Identity Provider username, because of the the
	// limited character set available for naming (see RFC1123) in K8s. If the username contains characters which are
	// disqualified from the resource name, the username is transformed into an acceptable resource name instead.
	// For example, johnsmith@redhat.com -> johnsmith-at-redhat-com
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
	IsTestingMode() bool
}

// Service represents the signup service for controllers.
type Service interface {
	GetSignup(userID string) (*Signup, error)
	CreateUserSignup(username, userID string) (*crtapi.UserSignup, error)
}

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct {
	Namespace         string
	UserSignups       kubeclient.UserSignupInterface
	MasterUserRecords kubeclient.MasterUserRecordInterface
}

// NewSignupService creates a service object for performing user signup-related activities.
func NewSignupService(cfg ServiceConfiguration) (Service, error) {

	if cfg.IsTestingMode() {
		// testing mode, return default impl instance. This is needed
		// for server and middleware tests where we need a full server
		// initialization. In those cases, the mocking used in the
		// signup controller tests can not be used as the initialization
		// is happening before the test can hook into it.
		return &ServiceImpl{}, nil
	}

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
func (s *ServiceImpl) CreateUserSignup(username, userID string) (*crtapi.UserSignup, error) {
	name, err := s.transformAndValidateUserName(username)
	if err != nil {
		return nil, err
	}

	userSignup := &crtapi.UserSignup{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: s.Namespace,
		},
		Spec: crtapi.UserSignupSpec{
			UserID:        userID,
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
		Username:      userSignup.Spec.Username,
		TargetCluster: userSignup.Spec.TargetCluster,
	}

	// Check UserSignup status to determine whether user signup is complete
	signupCondition, complete := condition.FindConditionByType(userSignup.Status.Conditions, crtapi.UserSignupComplete)
	if !complete {
		signupResponse.Status = Status{
			Reason:  signupCondition.Reason,
			Message: signupCondition.Message,
		}
		return signupResponse, nil
	}
	// If UserSignup status is complete, retrieve MasterUserRecord resource from the host cluster and use its status
	mur, err := s.MasterUserRecords.Get(userSignup.GetName())
	if err != nil {
		return nil, errors2.Wrap(err, fmt.Sprintf("error when retrieving MasterUserRecord for completed UserSignup %s", userSignup.GetName()))
	}
	murCondition, ready := condition.FindConditionByType(mur.Status.Conditions, crtapi.ConditionReady)
	signupResponse.Status = Status{
		Ready:   ready,
		Reason:  murCondition.Reason,
		Message: murCondition.Message,
	}

	return signupResponse, nil
}

func (s *ServiceImpl) transformAndValidateUserName(username string) (string, error) {
	replaced := strings.ReplaceAll(strings.ReplaceAll(username, "@", "-at-"), ".", "-")

	errs := validation.IsQualifiedName(replaced)
	if len(errs) > 0 {
		return "", errors2.New(fmt.Sprintf("Transformed username [%s] is invalid", username))
	}

	iteration := 0
	transformed := replaced

	for {
		userSignup, err := s.UserSignups.Get(transformed)
		if err != nil {
			if !errors.IsNotFound(err) {
				return "", err
			}
		}

		if userSignup == nil {
			break
		}

		iteration++
		transformed = fmt.Sprintf("%s-%d", replaced, iteration)
	}

	return transformed, nil
}
