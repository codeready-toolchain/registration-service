package informers

import (
	"fmt"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient/resources"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Informer struct {
	Masteruserrecord cache.GenericLister
	Space            cache.GenericLister
	SpaceBinding     cache.GenericLister
	ToolchainStatus  cache.GenericLister
	UserSignup       cache.GenericLister
}

func StartInformer(cfg *rest.Config) (*Informer, chan struct{}, error) {

	informer := &Informer{}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, 0, configuration.Namespace(), nil)

	// MasterUserRecords
	genericMasterUserRecordInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: resources.MurResourcePlural})
	informer.Masteruserrecord = genericMasterUserRecordInformer.Lister()
	masterUserRecordInformer := genericMasterUserRecordInformer.Informer()

	// Space
	genericSpaceInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: resources.SpaceResourcePlural})
	informer.Space = genericSpaceInformer.Lister()
	spaceInformer := genericSpaceInformer.Informer()

	// Space
	genericSpaceBindingInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: resources.SpaceBindingResourcePlural})
	informer.SpaceBinding = genericSpaceBindingInformer.Lister()
	spaceBindingInformer := genericSpaceBindingInformer.Informer()

	// ToolchainStatus
	genericToolchainStatusInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: resources.ToolchainStatusPlural})
	informer.ToolchainStatus = genericToolchainStatusInformer.Lister()
	toolchainstatusInformer := genericToolchainStatusInformer.Informer()

	// UserSignups
	genericUserSignupInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: resources.UserSignupResourcePlural})
	informer.UserSignup = genericUserSignupInformer.Lister()
	userSignupInformer := genericUserSignupInformer.Informer()

	stopper := make(chan struct{})

	log.Info(nil, "Starting proxy cache informers")
	factory.Start(stopper)

	if !cache.WaitForCacheSync(stopper,
		masterUserRecordInformer.HasSynced,
		spaceInformer.HasSynced,
		spaceBindingInformer.HasSynced,
		toolchainstatusInformer.HasSynced,
		userSignupInformer.HasSynced,
	) {
		err := fmt.Errorf("timed out waiting for caches to sync")
		log.Error(nil, err, "Failed to create informers")
		return nil, nil, err
	}
	log.Info(nil, "Informer caches synced")

	return informer, stopper, nil
}
