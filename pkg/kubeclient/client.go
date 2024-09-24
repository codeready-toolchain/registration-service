package kubeclient

import (
	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
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
func NewCRTRESTClient(client client.Client, namespace string) (CRTClient, error) {
	scheme := runtime.NewScheme()
	err := crtapi.SchemeBuilder.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	crtRESTClient := &CRTRESTClient{
		Client: client,
		NS:     namespace,
		Scheme: scheme,
	}

	crtRESTClient.v1Alpha1 = &V1Alpha1REST{client: crtRESTClient}
	return crtRESTClient, nil
}

type CRTRESTClient struct {
	Client   client.Client
	NS       string
	Scheme   *runtime.Scheme
	v1Alpha1 *V1Alpha1REST
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
			client: c.client.Client,
			ns:     c.client.NS,
			scheme: c.client.Scheme,
		},
	}
}

// MasterUserRecords returns an interface which may be used to perform CRUD operations for MasterUserRecord resources
func (c *V1Alpha1REST) MasterUserRecords() MasterUserRecordInterface {
	return &masterUserRecordClient{
		crtClient: crtClient{
			client: c.client.Client,
			ns:     c.client.NS,
			scheme: c.client.Scheme,
		},
	}
}

// BannedUsers returns an interface which may be used to perform query operations on BannedUser resources
func (c *V1Alpha1REST) BannedUsers() BannedUserInterface {
	return &bannedUserClient{
		crtClient: crtClient{
			client: c.client.Client,
			ns:     c.client.NS,
			scheme: c.client.Scheme,
		},
	}
}

// ToolchainStatuses returns an interface which may be used to perform query operations on ToolchainStatus resources
func (c *V1Alpha1REST) ToolchainStatuses() ToolchainStatusInterface {
	return &toolchainStatusClient{
		crtClient: crtClient{
			client: c.client.Client,
			ns:     c.client.NS,
			scheme: c.client.Scheme,
		},
	}
}

// SocialEvents returns an interface which may be used to perform CRUD operations for SocialEvent resources
func (c *V1Alpha1REST) SocialEvents() SocialEventInterface {
	return &socialeventClient{
		crtClient: crtClient{
			client: c.client.Client,
			ns:     c.client.NS,
			scheme: c.client.Scheme,
		},
	}
}

// Spaces returns an interface which may be used to perform CRUD operations for Space resources
func (c *V1Alpha1REST) Spaces() SpaceInterface {
	return &spaceClient{
		crtClient: crtClient{
			client: c.client.Client,
			ns:     c.client.NS,
			scheme: c.client.Scheme,
		},
	}
}

// SpaceBindings returns an interface which may be used to perform CRUD operations for SpaceBinding resources
func (c *V1Alpha1REST) SpaceBindings() SpaceBindingInterface {
	return &spaceBindingClient{
		crtClient: crtClient{
			client: c.client.Client,
			ns:     c.client.NS,
			scheme: c.client.Scheme,
		},
	}
}

type crtClient struct {
	client client.Client
	ns     string
	scheme *runtime.Scheme
}
