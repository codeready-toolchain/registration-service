package kubeclient

import (
	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/informers"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

type CRTClient interface {
	V1Alpha1() V1Alpha1
}

type V1Alpha1 interface {
	UserSignups() UserSignupInterface
	MasterUserRecords() MasterUserRecordInterface
	BannedUsers() BannedUserInterface
	ToolchainStatuses() ToolchainStatusInterface
	SocialEvents() SocialEventInterface
	Spaces() SpaceInterface
	SpaceBindings() SpaceBindingInterface
}

// NewCRTRESTClient creates a new REST client for managing Codeready Toolchain resources via the Kubernetes API
func NewCRTRESTClient(cfg *rest.Config, informer informers.Informer, namespace string) (CRTClient, error) {
	scheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	crtapi.SchemeBuilder.Register(getRegisterObject()...)

	config := *cfg
	config.GroupVersion = &crtapi.GroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	restClient, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	crtRESTClient := &CRTRESTClient{
		RestClient: restClient,
		Informer:   informer,
		Config:     config,
		NS:         namespace,
		Scheme:     scheme,
	}

	crtRESTClient.v1Alpha1 = &V1Alpha1REST{client: crtRESTClient}
	return crtRESTClient, nil
}

func getRegisterObject() []runtime.Object {
	return []runtime.Object{
		&crtapi.UserSignup{},
		&crtapi.UserSignupList{},
		&crtapi.MasterUserRecord{},
		&crtapi.MasterUserRecordList{},
		&crtapi.BannedUser{},
		&crtapi.BannedUserList{},
		&crtapi.ToolchainStatus{},
		&crtapi.ToolchainStatusList{},
		&crtapi.Space{},
		&crtapi.SpaceList{},
		&crtapi.SpaceBinding{},
		&crtapi.SpaceBindingList{},
	}
}

type CRTRESTClient struct {
	RestClient rest.Interface
	Informer   informers.Informer
	NS         string
	Config     rest.Config
	Scheme     *runtime.Scheme
	v1Alpha1   *V1Alpha1REST
}

func (c *CRTRESTClient) V1Alpha1() V1Alpha1 {
	return c.v1Alpha1
}

type V1Alpha1REST struct {
	client *CRTRESTClient
}

// UserSignups returns an interface which may be used to perform CRUD operations for UserSignup resources
func (c *V1Alpha1REST) UserSignups() UserSignupInterface {
	return &userSignupClient{
		crtClient: crtClient{
			restClient: c.client.RestClient,
			informer:   c.client.Informer,
			ns:         c.client.NS,
			cfg:        c.client.Config,
			scheme:     c.client.Scheme,
		},
	}
}

// MasterUserRecords returns an interface which may be used to perform CRUD operations for MasterUserRecord resources
func (c *V1Alpha1REST) MasterUserRecords() MasterUserRecordInterface {
	return &masterUserRecordClient{
		crtClient: crtClient{
			restClient: c.client.RestClient,
			informer:   c.client.Informer,
			ns:         c.client.NS,
			cfg:        c.client.Config,
			scheme:     c.client.Scheme,
		},
	}
}

// BannedUsers returns an interface which may be used to perform query operations on BannedUser resources
func (c *V1Alpha1REST) BannedUsers() BannedUserInterface {
	return &bannedUserClient{
		crtClient: crtClient{
			restClient: c.client.RestClient,
			informer:   c.client.Informer,
			ns:         c.client.NS,
			cfg:        c.client.Config,
			scheme:     c.client.Scheme,
		},
	}
}

// ToolchainStatuses returns an interface which may be used to perform query operations on ToolchainStatus resources
func (c *V1Alpha1REST) ToolchainStatuses() ToolchainStatusInterface {
	return &toolchainStatusClient{
		crtClient: crtClient{
			restClient: c.client.RestClient,
			informer:   c.client.Informer,
			ns:         c.client.NS,
			cfg:        c.client.Config,
			scheme:     c.client.Scheme,
		},
	}
}

// SocialEvents returns an interface which may be used to perform CRUD operations for SocialEvent resources
func (c *V1Alpha1REST) SocialEvents() SocialEventInterface {
	return &socialeventClient{
		crtClient: crtClient{
			restClient: c.client.RestClient,
			informer:   c.client.Informer,
			ns:         c.client.NS,
			cfg:        c.client.Config,
			scheme:     c.client.Scheme,
		},
	}
}

// Spaces returns an interface which may be used to perform CRUD operations for Space resources
func (c *V1Alpha1REST) Spaces() SpaceInterface {
	return &spaceClient{
		crtClient: crtClient{
			restClient: c.client.RestClient,
			informer:   c.client.Informer,
			ns:         c.client.NS,
			cfg:        c.client.Config,
			scheme:     c.client.Scheme,
		},
	}
}

// SpaceBindings returns an interface which may be used to perform CRUD operations for SpaceBinding resources
func (c *V1Alpha1REST) SpaceBindings() SpaceBindingInterface {
	return &spaceBindingClient{
		crtClient: crtClient{
			restClient: c.client.RestClient,
			informer:   c.client.Informer,
			ns:         c.client.NS,
			cfg:        c.client.Config,
			scheme:     c.client.Scheme,
		},
	}
}

type crtClient struct {
	restClient rest.Interface
	informer   informers.Informer
	ns         string
	cfg        rest.Config
	scheme     *runtime.Scheme
}
