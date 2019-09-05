package signup

import (
	"encoding/json"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

// SignupService implements the signup endpoint.
type SignupService struct {
	config *configuration.Registry
	logger *log.Logger
}

// New returns a new healthService instance.
func New(logger *log.Logger, config *configuration.Registry) *SignupService {
	r := new(SignupService)
	r.logger = logger
	r.config = config
	return r
}

// HandleRequest returns a default heath check result.
func (srv *SignupService) HandleRequest(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), 500)
		return
	}
}
