package signup

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

// SignupService implements the signup endpoint, which is invoked for new user registrations.
type SignupService struct {
	config *configuration.Registry
	logger *log.Logger
	keyManager *KeyManager
}

// NewSignupService returns a new SignupService instance.
func NewSignupService(logger *log.Logger, config *configuration.Registry) (*SignupService, error) {
	service := &SignupService{
		logger: logger,
		config: config,
	}
	// create new KeyManager
	keyManager, err := NewKeyManager(logger, config)
	service.keyManager = keyManager
	if err != nil {
		return nil, err
	}
	return service, nil
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
