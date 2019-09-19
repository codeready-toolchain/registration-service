package signup

import (
	"context"
	"fmt"
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	//"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"strings"
)

func NewSignupClient() (*signupClientImpl, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	crtapi.SchemeBuilder.AddToScheme(scheme)
	crtapi.SchemeBuilder.Register(&crtapi.UserSignup{}, &crtapi.UserSignupList{})
	config.GroupVersion = &crtapi.SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}

	client, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}

	/*
		clientset, err := apiextension.NewForConfig(config)
		if err != nil {
			return nil, err
		}*/

	return &signupClientImpl{
		Clientset: client,
	}, nil
}

func (c *signupClientImpl) CreateUserSignup(ctx context.Context, username, userID string) (*crtapi.UserSignup, error) {
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

	created, err := c.Clientset.ApiextensionsV1().CustomResourceDefinitions().Create(userSignup.(*apiextensionv1.CustomResourceDefinition))
	if err != nil {
		return nil, err
	}

	return created.(*crtapi.UserSignup), nil
}

func (c *signupClientImpl) transformAndValidateUserName(username string) (string, error) {
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

func (c *signupClientImpl) getUserSignup(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}

	err := c.Client.
		Get().
		Resource("UserSignup").
		Do().
		Into(result)

	if err != nil {
		return nil, err
	}

	return result, nil
}
