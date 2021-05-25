package kubeclient

import (
	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"

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
}

// NewCRTRESTClient creates a new REST client for managing Codeready Toolchain resources via the Kubernetes API
func NewCRTRESTClient(cfg *rest.Config, namespace string) (CRTClient, error) {
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

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	crtRESTClient := &CRTRESTClient{
		RestClient: client,
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
	}
}

type CRTRESTClient struct {
	RestClient rest.Interface
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
			client: c.client.RestClient,
			ns:     c.client.NS,
			cfg:    c.client.Config,
			scheme: c.client.Scheme,
		},
	}
}

// MasterUserRecords returns an interface which may be used to perform CRUD operations for MasterUserRecord resources
func (c *V1Alpha1REST) MasterUserRecords() MasterUserRecordInterface {
	return &masterUserRecordClient{
		crtClient: crtClient{
			client: c.client.RestClient,
			ns:     c.client.NS,
			cfg:    c.client.Config,
			scheme: c.client.Scheme,
		},
	}
}

// BannedUsers returns an interface which may be used to perform query operations on BannedUser resources
func (c *V1Alpha1REST) BannedUsers() BannedUserInterface {
	return &bannedUserClient{
		crtClient: crtClient{
			client: c.client.RestClient,
			ns:     c.client.NS,
			cfg:    c.client.Config,
			scheme: c.client.Scheme,
		},
	}
}

// ToolchainStatuses returns an interface which may be used to perform query operations on ToolchainStatus resources
func (c *V1Alpha1REST) ToolchainStatuses() ToolchainStatusInterface {
	return &toolchainStatusClient{
		crtClient: crtClient{
			client: c.client.RestClient,
			ns:     c.client.NS,
			cfg:    c.client.Config,
			scheme: c.client.Scheme,
		},
	}
}

type crtClient struct {
	client rest.Interface
	ns     string
	cfg    rest.Config
	scheme *runtime.Scheme
}
