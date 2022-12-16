package proxy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// TODO consolidate constants with signup.go
const (
	spaceResourcePlural            = "spaces"
	toolchainclusterResourcePlural = "toolchainclusters"
	usersignupResourcePlural       = "usersignups"
)

type CacheInvalidator struct {
	cfg       *rest.Config
	userCache *UserAccess
}

type Listers struct {
	userSignup       cache.GenericLister
	space            cache.GenericLister
	toolchainCluster cache.GenericLister
}

func (c *CacheInvalidator) Start() (*Listers, chan struct{}, error) {

	listers := &Listers{}
	dynamicClient, err := dynamic.NewForConfig(c.cfg)
	if err != nil {
		return nil, nil, err
	}

	log.Info(nil, "Creating an informer for "+spaceResourcePlural+" in namespace "+configuration.Namespace())
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, time.Hour, configuration.Namespace(), nil)

	// Spaces
	genericSpaceInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: spaceResourcePlural})
	listers.space = genericSpaceInformer.Lister()
	spaceInformer := genericSpaceInformer.Informer()
	spaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.onUpdate,
		DeleteFunc: c.onDelete,
	})

	// UserSignups
	genericSpaceInformer := factory.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: spaceResourcePlural})
	listers.space = genericSpaceInformer.Lister()
	spaceInformer := genericSpaceInformer.Informer()
	spaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.onUpdate,
		DeleteFunc: c.onDelete,
	})

	// ToolchainClusters

	stopper := make(chan struct{})

	log.Info(nil, "Starting proxy cache invalidator")
	factory.Start(stopper)

	if !cache.WaitForCacheSync(stopper, spaceInformer.HasSynced) {
		err := fmt.Errorf("Timed out waiting for caches to sync")
		log.Error(nil, err, "Failed to create informer")
		return spaceLister, nil, err
	}

	return spaceLister, stopper, nil
}

func (c *CacheInvalidator) onUpdate(oldObj interface{}, newObj interface{}) {
	previousObj := oldObj.(*unstructured.Unstructured)
	currentObj := newObj.(*unstructured.Unstructured)
	spaceName := currentObj.GetName()

	var previousSpace toolchainv1alpha1.Space
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(previousObj.UnstructuredContent(), previousSpace); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for Space '%s' due to previous object conversion", spaceName)
		return
	}

	var currentSpace toolchainv1alpha1.Space
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(currentObj.UnstructuredContent(), currentSpace); err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for Space '%s' due to current object conversion", spaceName)
		return
	}

	if previousSpace.Spec.TargetCluster != currentSpace.Spec.TargetCluster {
		log.Infof(nil, "Space '%s' target cluster has changed, proceed to invalidate cache", spaceName)
		go invalidateCacheForSpace(currentSpace.DeepCopy(), c.userCache)
	}

	// log.Info(nil, "Resource updated -> ")
}

func (c *CacheInvalidator) onDelete(obj interface{}) {
	us := obj.(*unstructured.Unstructured)
	spaceName := us.GetName()
	log.Info(nil, "Space deleted -> "+spaceName)
	// c.userCache.Invalidate(signupName)
	// log.Info(nil, "Resource deleted -> ")
}

func invalidateCacheForSpace(space *toolchainv1alpha1.Space, userCache *UserAccess) {
	cl, err := newClusterClient()
	if err != nil {
		log.Errorf(nil, err, "failed to invalidate cache for Space '%s'", space.Name)
		return
	}

	// TODO add retries

	// lookup SpaceBindings
	spaceBindings := toolchainv1alpha1.SpaceBindingList{}
	if err := cl.List(context.TODO(),
		&spaceBindings,
		client.InNamespace(space.Namespace),
		client.MatchingLabels{
			toolchainv1alpha1.SpaceBindingSpaceLabelKey: space.Name,
		},
	); err != nil {
		log.Error(nil, err, "failed to list space bindings")
	}

	// lookup MURs and get UserSignup names from each MUR
	userSignups := []string{}
	for _, sb := range spaceBindings.Items {
		murName := sb.Labels[toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey]
		mur := &toolchainv1alpha1.MasterUserRecord{}
		if err := cl.Get(context.TODO(), types.NamespacedName{Namespace: space.Namespace, Name: murName}, mur); err != nil {
			log.Error(nil, err, "failed to list space bindings")
		}
		log.Infof(nil, "Cache invalidator get mur: '%s'", mur.Name)
	}

	log.Infof(nil, "Cache invalidated for users: '%s'", strings.Join(userSignups, ","))
}
