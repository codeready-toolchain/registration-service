package controller

import (
	"encoding/json"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	"github.com/gin-gonic/gin"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	config *configuration.Registry
}

// NewSignup returns a new Signup instance.
func NewSignup(config *configuration.Registry) *Signup {
	return &Signup{
		config: config,
	}
}

// PostHandler returns signup info.
func (s *Signup) PostHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	// the KeyManager can be accessed here: auth.DefaultKeyManager()

	err := json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		log.Println(nil, "error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}
