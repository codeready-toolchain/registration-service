package fake

import (
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	log = logf.Log.WithName("fake-usersignup-client")
)

type FakeCRTV1Alpha1Client struct {
	NS string
}

func NewFakeCRTV1Alpha1Client(namespace string) *FakeCRTV1Alpha1Client {
	return &FakeCRTV1Alpha1Client{
		NS: namespace,
	}
}

func (c *FakeCRTV1Alpha1Client) UserSignups() kubeclient.UserSignupInterface {
	return NewFakeUserSignupClient(c.NS)
}

func (c *FakeCRTV1Alpha1Client) MasterUserRecords() kubeclient.MasterUserRecordInterface {
	return NewFakeMasterUserRecordClient(c.NS)
}

func (c *FakeCRTV1Alpha1Client) ToolchainStatuses() kubeclient.ToolchainStatusInterface {
	return NewFakeToolchainStatusClient(c.NS)
}

// getGVRFromObject returns the GroupVersionResource for the specified object and scheme
func getGVRFromObject(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionResource, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	return gvr, nil
}
