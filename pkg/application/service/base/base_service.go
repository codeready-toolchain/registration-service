package base

import "github.com/codeready-toolchain/registration-service/pkg/application/service/context"

type BaseService struct { // nolint: golint
	context.ServiceContext
}

func NewBaseService(context context.ServiceContext) BaseService {
	return BaseService{context}
}
