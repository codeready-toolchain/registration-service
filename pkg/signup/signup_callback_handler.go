package signup

import (
	"encoding/json"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

// SignupCallbackService implements the signup callback endpoint.
type SignupCallbackService struct {
	config *configuration.Registry
	logger *log.Logger
}

// NewSignupCallbackService returns a new SignupCallbackService instance.
func NewSignupCallbackService(logger *log.Logger, config *configuration.Registry) *SignupCallbackService {
	return &SignupCallbackService{
		logger: logger,
		config: config,
	}
}

// HandleRequest returns a default heath check result.
func (srv *SignupCallbackService) HandleRequest(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), 500)
		return
	}
}
