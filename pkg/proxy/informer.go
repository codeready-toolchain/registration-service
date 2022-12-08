package proxy

import (
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	crtapi "github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// TODO consolidate constants with signup.go
const (
	userSignupResourcePlural = "usersignups"
	byUserSignupNameIndexKey = "usersignup-name"
)

func startCacheInvalidator(cfg *rest.Config) (chan struct{}, error) {

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	log.Info(nil, "Create an informer for UserSignups in namespace "+configuration.Namespace())
	informer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, 10*time.Hour, configuration.Namespace(), nil)
	userSignupInformer := informer.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: userSignupResourcePlural}).Informer()
	userSignupInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onUserSignupAdd,
		UpdateFunc: onUserSignupUpdate,
		DeleteFunc: onUserSignupDelete,
	})

	stopper := make(chan struct{})

	log.Info(nil, "Starting proxy cache invalidator")
	informer.Start(stopper)
	// log.Info(nil, "Wait for informer cache to sync")
	// informer.WaitForCacheSync(stopper)
	return stopper, nil
}

// when a new pod is deployed the onAdd function would be invoked
// for now just print the event.
func onUserSignupAdd(obj interface{}) {
	us := obj.(*crtapi.UserSignup)
	signupName := us.GetName()
	log.Info(nil, "UserSignup added -> "+signupName)
}

// when a pod is deleted the onDelete function would be invoked
// for now just print the event
func onUserSignupUpdate(oldObj interface{}, newObj interface{}) {
	us := newObj.(*crtapi.UserSignup)
	signupName := us.GetName()
	log.Info(nil, "UserSignup updated -> "+signupName)
}

func onUserSignupDelete(obj interface{}) {
	us := obj.(*crtapi.UserSignup)
	signupName := us.GetName()
	log.Info(nil, "UserSignup deleted -> "+signupName)
}
