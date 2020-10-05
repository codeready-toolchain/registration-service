package base

import "github.com/codeready-toolchain/registration-service/pkg/application/service/context"

type BaseService struct {
	context.ServiceContext
}

func NewBaseService(context context.ServiceContext) BaseService {
	return BaseService{context}
}
