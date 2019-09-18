package controller

import (
	"encoding/json"
	"log"
	"net/http"

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

// PostSignupHandler returns signup info.
func (srv *Signup) PostSignupHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
