package controller

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/gin-gonic/gin"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	config *configuration.Registry
	logger *log.Logger
}

// NewSignup returns a new Signup instance.
func NewSignup(logger *log.Logger, config *configuration.Registry) *Signup {
	return &Signup{
		logger: logger,
		config: config,
	}
}

// PostHandler returns signup info.
func (s *Signup) PostHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	// the KeyManager can be accessed here: auth.DefaultKeyManager()

	err := json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		s.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}
