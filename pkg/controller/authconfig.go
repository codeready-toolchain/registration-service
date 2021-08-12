package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
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
}

// NewAuthConfig returns a new AuthConfig instance.
func NewAuthConfig() *AuthConfig {
	return &AuthConfig{}
}

// GetHandler returns raw auth config content for UI.
func (ac *AuthConfig) GetHandler(ctx *gin.Context) {
	cfg := commonconfig.GetCachedToolchainConfig()
	configRespData := configResponse{
		AuthClientLibraryURL: cfg.RegistrationService().Auth().AuthClientLibraryURL(),
		AuthClientConfigRaw:  cfg.RegistrationService().Auth().AuthClientConfigRaw(),
	}
	ctx.JSON(http.StatusOK, configRespData)
}
