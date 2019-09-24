package controller

import (
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

type configResponse struct {
	AuthClientLibraryURL string `json:"auth-client-library-url"`
	// this holds the raw config. Note: this is intentionally a string
	// not json as this field may also hold non-json configs!
	AuthClientConfigRaw string `json:"auth-client-config"`
}

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

// GetHandler returns raw auth config content for UI.
func (ac *AuthConfig) GetHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", ac.config.GetAuthClientConfigAuthContentType())
	ctx.Writer.WriteHeader(http.StatusOK)
<<<<<<< HEAD
	configRespData := configResponse {
		AuthClientLibraryURL: ac.config.GetAuthClientLibraryURL(),
		AuthClientConfigRaw: ac.config.GetAuthClientConfigAuthRaw(),
	}
	ctx.JSON(http.StatusOK, configRespData)
=======
	_, err := ctx.Writer.WriteString(ac.config.GetAuthClientConfigAuthRaw())
	if err != nil {
		ac.logger.Println("error writing response body", err.Error())
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
>>>>>>> 3-middleware
}
