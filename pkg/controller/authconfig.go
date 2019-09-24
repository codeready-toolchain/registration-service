package controller

import (
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/gin-gonic/gin"
)

// AuthConfig implements the auth config endpoint, which is invoked to
// retrieve the auth config for the ui.
type AuthConfig struct {
	config *configuration.Registry
	logger *log.Logger
}

// NewAuthConfig returns a new AuthConfig instance.
func NewAuthConfig(logger *log.Logger, config *configuration.Registry) *AuthConfig {
	return &AuthConfig{
		logger: logger,
		config: config,
	}
}

// AuthconfigHandler returns raw auth config content for UI.
func (ac *AuthConfig) GetHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", ac.config.GetAuthClientConfigAuthContentType())
	ctx.Writer.WriteHeader(http.StatusOK)
	_, err := ctx.Writer.WriteString(ac.config.GetAuthClientConfigAuthRaw())
	if err != nil {
		ac.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}
