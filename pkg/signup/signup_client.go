package signup

import (
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	// See note below
	//v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	userSignupResourcePlural = "usersignups"
)

func NewUserSignupClient(cfg *rest.Config) (*UserSignupV1Alpha1Client, error) {
	scheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	crtapi.SchemeBuilder.Register(&crtapi.UserSignup{}, &crtapi.UserSignupList{})

	config := *cfg
	config.GroupVersion = &crtapi.SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}
	config.UserAgent = rest.DefaultKubernetesUserAgent()

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

	// See note below
	//Update(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
	//Delete(name string, options *v1.DeleteOptions) error
}

type userSignupClientImpl struct {
	client rest.Interface
	ns     string
}

func (c *userSignupClientImpl) Get(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Name(name).
		Do().
		Into(result)
	return result, err
}

func (c *userSignupClientImpl) Create(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Body(obj).
		Do().
		Into(result)
	return result, err
}

/*

// DO NOT REMOVE - while these functions are not currently required by registration service, we may decide to migrate
// the UserSignupClient implementation to the common package.  In this case, all CRUD operations should be available
// and so this commented code should remain as an example of how to provide complete CRUD functionality.

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
*/
