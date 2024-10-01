package context

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceContextProducer func() ServiceContext

type ServiceContext interface {
	CRTClient() kubeclient.CRTClient
	Client() client.Client
	Services() service.Services
}
