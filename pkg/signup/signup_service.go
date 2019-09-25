package signup

import (
	"context"
	"fmt"
	"strings"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"

	errors2 "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
)

type SignupServiceConfiguration interface {
	GetNamespace() string
}

type SignupService interface {
	CreateUserSignup(ctx context.Context, username, userID string) (*crtapi.UserSignup, error)
}

type SignupServiceImpl struct {
	Namespace string
	Client    kubeclient.CRTClientInterface
}

func NewSignupService(cfg SignupServiceConfiguration) (SignupService, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubeclient.NewCRTV1Alpha1Client(k8sConfig, cfg.GetNamespace())
	if err != nil {
		return nil, err
	}

	return &SignupServiceImpl{
		Namespace: cfg.GetNamespace(),
		Client:    client.Client(),
	}, nil
}

func (c *SignupServiceImpl) CreateUserSignup(ctx context.Context, username, userID string) (*crtapi.UserSignup, error) {
	name, err := c.transformAndValidateUserName(username)
	if err != nil {
		return nil, err
	}

	userSignup := &crtapi.UserSignup{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: c.Namespace,
		},
		Spec: crtapi.UserSignupSpec{
			UserID:        userID,
			TargetCluster: "",
			Approved:      false,
			Username:      username,
		},
	}

	created, err := c.Client.CreateUserSignup(userSignup)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (c *SignupServiceImpl) transformAndValidateUserName(username string) (string, error) {
	replaced := strings.ReplaceAll(strings.ReplaceAll(username, "@", "-at-"), ".", "-")

	errs := validation.IsQualifiedName(replaced)
	if len(errs) > 0 {
		return "", errors2.New(fmt.Sprintf("Transformed username [%s] is invalid", username))
	}

	iteration := 0
	transformed := replaced

	for {
		userSignup, err := c.Client.GetUserSignup(transformed)
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
