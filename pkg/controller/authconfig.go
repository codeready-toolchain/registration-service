package controller

import (
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
}

// NewAuthConfig returns a new AuthConfig instance.
func NewAuthConfig(config *configuration.Registry) *AuthConfig {
	return &AuthConfig{
		config: config,
	}
}

// GetHandler returns raw auth config content for UI.
func (ac *AuthConfig) GetHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")
	configRespData := configResponse{
		AuthClientLibraryURL: ac.config.GetAuthClientLibraryURL(),
		AuthClientConfigRaw:  ac.config.GetAuthClientConfigAuthRaw(),
	}
	ctx.JSON(http.StatusOK, configRespData)
}
