package kubeclient

import (
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

const (
	userSignupResourcePlural = "usersignups"
)

func NewCRTV1Alpha1Client(cfg *rest.Config, namespace string) (*CRTV1Alpha1Client, error) {
	scheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	crtapi.SchemeBuilder.Register(getRegisterObject()...)

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

	return &CRTV1Alpha1Client{
		RestClient: client,
		NS:         namespace,
	}, nil
}

func getRegisterObject() []runtime.Object {
	return []runtime.Object{&crtapi.UserSignup{}, &crtapi.UserSignupList{}}
}

type CRTV1Alpha1Client struct {
	RestClient rest.Interface
	NS         string
}

func (c *CRTV1Alpha1Client) Client() CRTClientInterface {
	return &crtClient{
		client: c.RestClient,
		ns:     c.NS,
	}
}

type crtClient struct {
	client rest.Interface
	ns     string
}

type CRTClientInterface interface {
	GetUserSignup(name string) (*crtapi.UserSignup, error)
	CreateUserSignup(obj *crtapi.UserSignup) (*crtapi.UserSignup, error)
}

func (c *crtClient) GetUserSignup(name string) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Get().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Name(name).
		Do().
		Into(result)
	return result, err
}

func (c *crtClient) CreateUserSignup(obj *crtapi.UserSignup) (*crtapi.UserSignup, error) {
	result := &crtapi.UserSignup{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource(userSignupResourcePlural).
		Body(obj).
		Do().
		Into(result)
	return result, err
}
