package signup

import (
	"context"
	"fmt"
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"strings"
)

type SignupServiceConfiguration interface {
	GetNamespace() string
}

type SignupService interface {
	CreateUserSignup(ctx context.Context, username, userID string) (*crtapi.UserSignup, error)
}

type SignupServiceImpl struct {
	Client UserSignupClient
}

func NewSignupService(cfg SignupServiceConfiguration) (SignupService, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := NewUserSignupClient(k8sConfig)
	if err != nil {
		return nil, err
	}

	return &SignupServiceImpl{
		Client: client.UserSignups(cfg.GetNamespace()),
	}, nil
}

func (c *SignupServiceImpl) CreateUserSignup(ctx context.Context, username, userID string) (*crtapi.UserSignup, error) {
	name, err := c.transformAndValidateUserName(username)
	if err != nil {
		return nil, err
	}

	userSignup := &crtapi.UserSignup{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Spec: crtapi.UserSignupSpec{
			UserID:        userID,
			TargetCluster: "",
			Approved:      false,
			Username:      username,
		},
	}

	created, err := c.Client.Create(userSignup)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (c *SignupServiceImpl) transformAndValidateUserName(username string) (string, error) {
	replaced := strings.ReplaceAll(strings.ReplaceAll(username, "@", "-at-"), ".", "-")

	iteration := 0
	transformed := replaced

	for {
		userSignup, err := c.Client.Get(transformed)
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
