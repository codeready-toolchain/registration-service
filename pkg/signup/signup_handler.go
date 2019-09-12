package signup

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// SignupService implements the signup endpoint, which is invoked for new user registrations.
type SignupService struct {
	config *configuration.Registry
	logger *log.Logger
}

// NewSignupService returns a new SignupService instance.
func NewSignupService(logger *log.Logger, config *configuration.Registry) *SignupService {
	return &SignupService{
		logger: logger,
		config: config,
	}
}

// PostSignupHandler returns signup info.
func (srv *SignupService) PostSignupHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
