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

type SignupService interface {
	CreateUserSignup(ctx context.Context, username, userID string) (*crtapi.UserSignup, error)
}

type signupServiceImpl struct {
	client UserSignupClient
}

func NewSignupService(namespace string) (SignupService, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := NewUserSignupClient(config)
	if err != nil {
		return nil, err
	}

	return &signupServiceImpl{
		client: client.UserSignups(namespace),
	}, nil
}

func (c *signupServiceImpl) CreateUserSignup(ctx context.Context, username, userID string) (*crtapi.UserSignup, error) {
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

	created, err := c.client.Create(userSignup)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (c *signupServiceImpl) transformAndValidateUserName(username string) (string, error) {
	replaced := strings.ReplaceAll(strings.ReplaceAll(username, "@", "-at-"), ".", "-")

	iteration := 0
	transformed := replaced

	for {
		userSignup, err := c.client.Get(transformed)
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
