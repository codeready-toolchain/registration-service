package controller

import (
	"log"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"

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
}
