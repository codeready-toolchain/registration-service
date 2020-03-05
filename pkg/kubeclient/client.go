package kubeclient

import (
	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

// NewCRTV1Alpha1Client creates a new REST client for managing Codeready Toolchain resources via the Kubernetes API
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
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &CRTV1Alpha1Client{
		RestClient: client,
		Config:     config,
		NS:         namespace,
		Scheme:     scheme,
	}, nil
}

func getRegisterObject() []runtime.Object {
	return []runtime.Object{
		&crtapi.UserSignup{},
		&crtapi.UserSignupList{},
		&crtapi.MasterUserRecord{},
		&crtapi.MasterUserRecordList{},
		&crtapi.BannedUser{},
		&crtapi.BannedUserList{},
	}
}

type CRTV1Alpha1Client struct {
	RestClient rest.Interface
	NS         string
	Config     rest.Config
	Scheme     *runtime.Scheme
}

// UserSignups returns an interface which may be used to perform CRUD operations for UserSignup resources
func (c *CRTV1Alpha1Client) UserSignups() UserSignupInterface {
	return &userSignupClient{
		crtClient: crtClient{
			client: c.RestClient,
			ns:     c.NS,
			cfg:    c.Config,
			scheme: c.Scheme,
		},
	}
}

// MasterUserRecords returns an interface which may be used to perform CRUD operations for MasterUserRecord resources
func (c *CRTV1Alpha1Client) MasterUserRecords() MasterUserRecordInterface {
	return &masterUserRecordClient{
		crtClient: crtClient{
			client: c.RestClient,
			ns:     c.NS,
			cfg:    c.Config,
			scheme: c.Scheme,
		},
	}
}

// BannedUsers returns an interface which may be used to perform query operations on BannedUser resources
func (c *CRTV1Alpha1Client) BannedUsers() BannedUserInterface {
	return &bannedUserClient{
		crtClient: crtClient{
			client: c.RestClient,
			ns:     c.NS,
			cfg:    c.Config,
			scheme: c.Scheme,
		},
	}
}

type crtClient struct {
	client rest.Interface
	ns     string
	cfg    rest.Config
	scheme *runtime.Scheme
}
