package signup

import (
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

const (
	USER_SIGNUP_RESOURCE_NAME = "usersignups"
)

func NewUserSignupClient(cfg *rest.Config) (*UserSignupV1Alpha1Client, error) {
	scheme := runtime.NewScheme()
	crtapi.SchemeBuilder.AddToScheme(scheme)
	crtapi.SchemeBuilder.Register(&crtapi.UserSignup{}, &crtapi.UserSignupList{})

	config := *cfg
	config.GroupVersion = &crtapi.SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &UserSignupV1Alpha1Client{
		restClient: client,
	}, nil
}

type UserSignupV1Alpha1Client struct {
	restClient rest.Interface
}

func (c *UserSignupV1Alpha1Client) UserSignups(namespace string) UserSignupClient {
	return &userSignupClientImpl{
		client: c.restClient,
		ns:     namespace,
	}
}

type UserSignupClient interface {
	Get(name string) (*crtapi.UserSignup, error)
	Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	Update(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	Delete(name string, options *v1.DeleteOptions) error
}

type userSignupClientImpl struct {
	client rest.Interface
	ns     string
}

func (c *userSignupClientImpl) Get(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(USER_SIGNUP_RESOURCE_NAME).
		Name(name).
		Do().
		Into(result)
	return result, err
}

func (c *userSignupClientImpl) Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource(USER_SIGNUP_RESOURCE_NAME).
		Body(obj).
		Do().
		Into(result)
	return result, err
}

func (c *userSignupClientImpl) Update(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Put().
		Namespace(c.ns).
		Resource(USER_SIGNUP_RESOURCE_NAME).
		Body(obj).
		Do().
		Into(result)
	return result, err
}

func (c *userSignupClientImpl) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource(USER_SIGNUP_RESOURCE_NAME).
		Name(name).
		Body(options).
		Do().
		Error()
}
