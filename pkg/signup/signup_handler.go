package signup

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// Service implements the signup endpoint, which is invoked for new user registrations.
type Service struct {
	config *configuration.Registry
	logger *log.Logger
}

// New returns a new Service instance.
func New(logger *log.Logger, config *configuration.Registry) *Service {
	return &Service{
		logger: logger,
		config: config,
	}
}

// PostSignupHandler returns signup info.
func (srv *Service) PostSignupHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
