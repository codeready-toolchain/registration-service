package informers

import (
	"fmt"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Informer struct {
	Masteruserrecord cache.GenericLister
	UserSignup       cache.GenericLister
	ToolchainStatus  cache.GenericLister
}

func StartInformer(cfg *rest.Config) (*Informer, chan struct{}, error) {

	informer := &Informer{}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, time.Hour, configuration.Namespace(), nil)

	// MasterUserRecords
	genericMasterUserRecordInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: kubeclient.MurResourcePlural})
	informer.Masteruserrecord = genericMasterUserRecordInformer.Lister()
	masterUserRecordInformer := genericMasterUserRecordInformer.Informer()

	// ToolchainStatus
	genericToolchainStatusInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: kubeclient.ToolchainStatusPlural})
	informer.ToolchainStatus = genericToolchainStatusInformer.Lister()
	toolchainstatusInformer := genericToolchainStatusInformer.Informer()

	// UserSignups
	genericUserSignupInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: kubeclient.UserSignupResourcePlural})
	informer.UserSignup = genericUserSignupInformer.Lister()
	userSignupInformer := genericUserSignupInformer.Informer()

	stopper := make(chan struct{})

	log.Info(nil, "Starting informers")
	factory.Start(stopper)

	if !cache.WaitForCacheSync(stopper, masterUserRecordInformer.HasSynced, userSignupInformer.HasSynced, toolchainstatusInformer.HasSynced) {
		err := fmt.Errorf("timed out waiting for caches to sync")
		log.Error(nil, err, "Failed to create informer")
		return nil, nil, err
	}
	log.Info(nil, "Informer caches synced")

	return informer, stopper, nil
}
