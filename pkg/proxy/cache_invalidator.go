package proxy

import (
	"fmt"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// TODO consolidate constants with signup.go
const (
	murResourcePlural              = "masteruserrecords"
	spaceResourcePlural            = "spaces"
	toolchainclusterResourcePlural = "toolchainclusters"
	userSignupResourcePlural       = "usersignups"
)

type Informer struct {
	masteruserrecord cache.GenericLister
	userSignup       cache.GenericLister
	space            cache.GenericLister
	toolchainCluster cache.GenericLister
}

func (inf *Informer) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	obj, err := inf.masteruserrecord.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)

	mur := &toolchainv1alpha1.MasterUserRecord{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), mur); err != nil {
		log.Errorf(nil, err, "failed to get MasterUserRecord '%s'", name)
		return nil, err
	}
	return mur, err
}

func (inf *Informer) GetSpace(name string) (*toolchainv1alpha1.Space, error) {
	obj, err := inf.space.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	space := &toolchainv1alpha1.Space{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), space); err != nil {
		log.Errorf(nil, err, "failed to get Space '%s'", name)
		return nil, err
	}

	return space, err
}

func (inf *Informer) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	obj, err := inf.userSignup.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	us := &toolchainv1alpha1.UserSignup{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), us); err != nil {
		log.Errorf(nil, err, "failed to get UserSignup '%s'", name)
		return nil, err
	}
	return us, err
}

func (inf *Informer) GetToolchainCluster(name string) (*toolchainv1alpha1.ToolchainCluster, error) {
	obj, err := inf.toolchainCluster.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	tc := &toolchainv1alpha1.ToolchainCluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), tc); err != nil {
		log.Errorf(nil, err, "failed to get ToolchainCluster '%s'", name)
		return nil, err
	}
	return tc, err
}

func StartInformer(cfg *rest.Config) (*Informer, chan struct{}, error) {

	informer := &Informer{}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	log.Info(nil, "Creating an informer for "+spaceResourcePlural+" in namespace "+configuration.Namespace())
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, time.Hour, configuration.Namespace(), nil)

	// MasterUserRecords
	genericMasterUserRecordInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: murResourcePlural})
	informer.masteruserrecord = genericMasterUserRecordInformer.Lister()
	masterUserRecordInformer := genericMasterUserRecordInformer.Informer()
	masterUserRecordInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: onMurUpdate,
		DeleteFunc: onMurDelete,
	})

	// Spaces
	genericSpaceInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: spaceResourcePlural})
	informer.space = genericSpaceInformer.Lister()
	spaceInformer := genericSpaceInformer.Informer()
	spaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: onSpaceUpdate,
		DeleteFunc: onSpaceDelete,
	})

	// UserSignups
	genericUserSignupInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: userSignupResourcePlural})
	informer.userSignup = genericUserSignupInformer.Lister()
	userSignupInformer := genericUserSignupInformer.Informer()
	userSignupInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: onUserSignupUpdate,
		DeleteFunc: onUserSignupDelete,
	})

	// ToolchainClusters
	genericToolchainClusterInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: toolchainclusterResourcePlural})
	informer.toolchainCluster = genericToolchainClusterInformer.Lister()
	ToolchainClusterInformer := genericToolchainClusterInformer.Informer()
	ToolchainClusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: onToolchainClusterUpdate,
		DeleteFunc: onToolchainClusterDelete,
	})

	stopper := make(chan struct{})

	log.Info(nil, "Starting proxy cache invalidator")
	factory.Start(stopper)

	if !cache.WaitForCacheSync(stopper, spaceInformer.HasSynced) {
		err := fmt.Errorf("timed out waiting for caches to sync")
		log.Error(nil, err, "Failed to create informer")
		return nil, nil, err
	}

	return informer, stopper, nil
}

func onMurUpdate(oldObj interface{}, newObj interface{}) {
	previousObj := oldObj.(*unstructured.Unstructured)
	currentObj := newObj.(*unstructured.Unstructured)
	murName := currentObj.GetName()

	var previousMur toolchainv1alpha1.MasterUserRecord
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(previousObj.UnstructuredContent(), &previousMur); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for MasterUserRecord '%s' due to previous object conversion", murName)
		return
	}

	var currentMur toolchainv1alpha1.MasterUserRecord
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(currentObj.UnstructuredContent(), &currentMur); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for MasterUserRecord '%s' due to current object conversion", murName)
		return
	}

	log.Infof(nil, "MasterUserRecord '%s' has changed", murName)

	// log.Info(nil, "Resource updated -> ")
}

func onMurDelete(obj interface{}) {
	mur := obj.(*unstructured.Unstructured)
	murName := mur.GetName()
	log.Info(nil, "MasterUserRecord deleted -> "+murName)
	// c.userCache.Invalidate(signupName)
	// log.Info(nil, "Resource deleted -> ")
}

func onSpaceUpdate(oldObj interface{}, newObj interface{}) {
	previousObj := oldObj.(*unstructured.Unstructured)
	currentObj := newObj.(*unstructured.Unstructured)
	spaceName := currentObj.GetName()

	var previousSpace toolchainv1alpha1.Space
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(previousObj.UnstructuredContent(), &previousSpace); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for Space '%s' due to previous object conversion", spaceName)
		return
	}

	var currentSpace toolchainv1alpha1.Space
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(currentObj.UnstructuredContent(), &currentSpace); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for Space '%s' due to current object conversion", spaceName)
		return
	}

	log.Infof(nil, "Space '%s' has changed", spaceName)

	// log.Info(nil, "Resource updated -> ")
}

func onSpaceDelete(obj interface{}) {
	us := obj.(*unstructured.Unstructured)
	spaceName := us.GetName()
	log.Info(nil, "Space deleted -> "+spaceName)
	// c.userCache.Invalidate(signupName)
	// log.Info(nil, "Resource deleted -> ")
}

func onUserSignupUpdate(oldObj interface{}, newObj interface{}) {
	previousObj := oldObj.(*unstructured.Unstructured)
	currentObj := newObj.(*unstructured.Unstructured)
	name := currentObj.GetName()

	var previousUserSignup toolchainv1alpha1.UserSignup
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(previousObj.UnstructuredContent(), &previousUserSignup); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for UserSignup '%s' due to previous object conversion", name)
		return
	}

	var currentUserSignup toolchainv1alpha1.UserSignup
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(currentObj.UnstructuredContent(), &currentUserSignup); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for UserSignup '%s' due to current object conversion", name)
		return
	}

	log.Infof(nil, "UserSignup '%s' has changed", name)

	// log.Info(nil, "Resource updated -> ")
}

func onUserSignupDelete(obj interface{}) {
	o := obj.(*unstructured.Unstructured)
	name := o.GetName()
	log.Info(nil, "UserSignup deleted -> "+name)
	// c.userCache.Invalidate(signupName)
	// log.Info(nil, "Resource deleted -> ")
}

func onToolchainClusterUpdate(oldObj interface{}, newObj interface{}) {
	previousObj := oldObj.(*unstructured.Unstructured)
	currentObj := newObj.(*unstructured.Unstructured)
	name := currentObj.GetName()

	var previousToolchainCluster toolchainv1alpha1.ToolchainCluster
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(previousObj.UnstructuredContent(), &previousToolchainCluster); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for ToolchainCluster '%s' due to previous object conversion", name)
		return
	}

	var currentToolchainCluster toolchainv1alpha1.ToolchainCluster
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(currentObj.UnstructuredContent(), &currentToolchainCluster); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for ToolchainCluster '%s' due to current object conversion", name)
		return
	}

	log.Infof(nil, "ToolchainCluster '%s' has changed", name)

	// log.Info(nil, "Resource updated -> ")
}

func onToolchainClusterDelete(obj interface{}) {
	o := obj.(*unstructured.Unstructured)
	name := o.GetName()
	log.Info(nil, "ToolchainCluster deleted -> "+name)
	// c.userCache.Invalidate(signupName)
	// log.Info(nil, "Resource deleted -> ")
}
