package proxy

import (
	"time"

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

	// r := schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: userSignupResourcePlural}
	// listOptions := metav1.ListOptions{
	// 	LabelSelector: fmt.Sprintf("%s!=%s", crtapi.UserSignupStateLabelKey, crtapi.UserSignupStateLabelValueDeactivated),
	// }

	informer := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 10*time.Hour)
	userSignupInformer := informer.ForResource(schema.GroupVersionResource{Group: "toolchain.dev.openshift.com", Version: "v1alpha1", Resource: userSignupResourcePlural})
	userSignupInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onUserSignupAdd,
		UpdateFunc: onUserSignupUpdate,
		DeleteFunc: onUserSignupDelete,
	})
	// userSignupList := indexer.ByIndex(byUserSignupNameIndexKey, )

	stopper := make(chan struct{})

	log.Info(nil, "Starting proxy cache invalidator")
	informer.Start(stopper)
	informer.WaitForCacheSync(stopper)

	// list, err := intf.Resource(r).Namespace(c.ns).List(context.TODO(), listOptions)
	// if err != nil {
	// 	return nil, err
	// }

	// result := &crtapi.UserSignupList{}

	// err = c.crtClient.scheme.Convert(list, result, nil)
	return stopper, nil
}

// func option2(clientset kubernetes.Interface, userAccessCache *UserAccess) {
// 	factory := informers.NewSharedInformerFactory(clientset, 0)
// 	informer := factory.Core().V1().Pods().Informer()
// 	stopper := make(chan struct{})
// 	defer close(stopper)
// 	defer runtime.HandleCrash()
// 	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
// 		AddFunc:    onAdd,
// 		DeleteFunc: onDelete,
// 	})
// 	go informer.Run(stopper)
// 	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
// 		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
// 		return
// 	}
// 	<-stopper
// }

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
