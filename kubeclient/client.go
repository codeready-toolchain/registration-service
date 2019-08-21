package kubeclient

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KubeClient struct {
	CoreClient kubernetes.Interface
}

func NewKubeClient(parameters []string) *KubeClient {
	var err error
	kc := new(KubeClient)
	config := getKubeConfig(parameters)
	kc.CoreClient, err = kubernetes.NewForConfig(&config)
	if err != nil {
		panic(err)
	}

	return kc
}

func getKubeConfig(parameters []string) rest.Config {
	host := parameters[0] + "://" + parameters[1] + ":" + parameters[2]
	bearerToken := parameters[3]

	return getOpenshiftAPIConfig(host, bearerToken)
}

func getOpenshiftAPIConfig(host string, bearerToken string) rest.Config {
	return rest.Config{
		Host:        host,
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}
