package signup

import (
	"fmt"
	"strings"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"

	errors2 "github.com/pkg/errors"
	"github.com/spf13/viper"
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
	Username string       `json:"username"`
	Status   SignupStatus `json:"status,omitempty"`
}

// SignupStatus represents UserSignup resource status
type SignupStatus struct {
	// If true then the corresponding user's account is ready to be used
	Ready bool `json:"ready"`
	// Brief reason for the status last transition.
	Reason string `json:"reason"`
	// Human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// SignupServiceConfiguration represents the config used for the signup service.
type SignupServiceConfiguration interface {
	GetNamespace() string
	IsTestingMode() bool
	GetViperInstance() *viper.Viper
}

// SignupService represents the signup service for controllers.
type SignupService interface {
	GetUserSignup(userID string) (*Signup, error)
	CreateUserSignup(username, userID string) (*crtapi.UserSignup, error)
}

// SignupServiceImpl represents the implementation of the signup service.
type SignupServiceImpl struct {
	Namespace   string
	UserSignups kubeclient.UserSignupInterface
	checkerFunc func(userID string) (*Signup, error)
}

// NewSignupService creates a service object for performing user signup-related activities.
func NewSignupService(cfg SignupServiceConfiguration) (SignupService, error) {

	if cfg.IsTestingMode() {
		// in testing mode, we mock the checker
		s := &SignupServiceImpl{
			Namespace:   cfg.GetNamespace(),
			UserSignups: nil,
		}
		checkerFunc := cfg.GetViperInstance().Get("checker")
		if checkerFunc != nil {
			s.checkerFunc = checkerFunc.(func(userID string) (*Signup, error))
		}
		return s, nil
	}

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubeclient.NewCRTV1Alpha1Client(k8sConfig, cfg.GetNamespace())
	if err != nil {
		return nil, err
	}

	return &SignupServiceImpl{
		Namespace:   cfg.GetNamespace(),
		UserSignups: client.UserSignups(),
	}, nil
}

// CreateUserSignup creates a new UserSignup resource with the specified username and userID
func (s *SignupServiceImpl) CreateUserSignup(username, userID string) (*crtapi.UserSignup, error) {
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

// getUserSignupImpl gets the UserSignup resource with the specified userID
// Returns nil, nil if the resource is not found
func (s *SignupServiceImpl) getUserSignupImpl(userID string) (*Signup, error) {
	// TODO
	/*
				us, err := c.UserSignups.Get(userID)
			    // TODO Check if signup exists. If yes then get the corresponding MUR and populate the status
		        // transform crt.UserSignup to signup.Signup
	*/
	return nil, nil
}

// GetUserSignup wraps getUserSignupImpl (or the mocked func)
func (s *SignupServiceImpl) GetUserSignup(userID string) (*Signup, error) {
	// this will call either getUserSignupImpl() (default) or a mocked func given by a test
	return s.checkerFunc(userID)
}

func (s *SignupServiceImpl) transformAndValidateUserName(username string) (string, error) {
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
