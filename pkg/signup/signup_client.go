package signup

import (
	"fmt"
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"
)

type signupClient struct {
	ClientSet *kubernetes.Clientset
}

func NewSignupClient() (*signupClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &signupClient{
		ClientSet: clientset,
	}, nil
}

func (c *signupClient) CreateUserSignup(username, userID string) error {
	name, err := c.transformAndValidateUserName(username)
	if err != nil {
		return err
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

	req := c.ClientSet.RESTClient().Post()
	req.Name("UserSignup")
	req.Body(userSignup)
	result := req.Do()

	return result.Error()
}

func (c *signupClient) transformAndValidateUserName(username string) (string, error) {
	replaced := strings.ReplaceAll(strings.ReplaceAll(username, "@", "-at-"), ".", "-")

	iteration := 0
	transformed := replaced

	for {
		userSignup, err := c.getUserSignup(transformed)
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

func (c *signupClient) getUserSignup(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}

	err := c.ClientSet.
		RESTClient().
		Get().
		Resource("UserSignup").
		Do().
		Into(result)

	if err != nil {
		return nil, err
	}

	return result, nil
}
