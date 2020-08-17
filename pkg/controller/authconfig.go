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
	AuthClientConfigRawRealm        string `json:"auth-client-config-realm"`
	AuthClientConfigRawServerURL    string `json:"auth-client-config-server-url"`
	AuthClientConfigRawSSLRequired  string `json:"auth-client-config-ssl-required"`
	AuthClientConfigRawResource     string `json:"auth-client-config-resource"`
	AuthClientConfigRawClientID     string `json:"auth-client-config-client-id"`
	AuthClientConfigRawPublicClient bool   `json:"auth-client-config-public-client"`
	VerificationEnabled             bool   `json:"verification-enabled"`
	VerificationDailyLimit          int    `json:"verification-daily-limit"`
	VerificationAttemptsAllowed     int    `json:"verification-attempts-allowed"`
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
	configRespData := configResponse{
		AuthClientLibraryURL:            ac.config.GetAuthClientLibraryURL(),
		AuthClientConfigRawClientID:     ac.config.GetAuthClientConfigAuthRawClientID(),
		AuthClientConfigRawPublicClient: ac.config.GetAuthClientConfigAuthRawPublicClient(),
		AuthClientConfigRawRealm:        ac.config.GetAuthClientConfigAuthRawRealm(),
		AuthClientConfigRawResource:     ac.config.GetAuthClientConfigAuthRawResource(),
		AuthClientConfigRawServerURL:    ac.config.GetAuthClientConfigAuthRawServerURL(),
		AuthClientConfigRawSSLRequired:  ac.config.GetAuthClientConfigAuthRawSSLReuired(),
		VerificationEnabled:             ac.config.GetVerificationEnabled(),
		VerificationDailyLimit:          ac.config.GetVerificationDailyLimit(),
		VerificationAttemptsAllowed:     ac.config.GetVerificationAttemptsAllowed(),
	}
	ctx.JSON(http.StatusOK, configRespData)
}
