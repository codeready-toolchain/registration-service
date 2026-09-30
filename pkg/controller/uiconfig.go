package controller

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// UIConfigResponse is the authenticated UI configuration.
// Values here require authentication.
type UIConfigResponse struct {
	WorkatoWebHookURL string `json:"workatoWebHookURL"`
}

// UIConfigPublicResponse is the public UI configuration.
// Values here are safe to return before login.
type UIConfigPublicResponse struct {
	DisabledIntegrations []string `json:"disabledIntegrations"`
}

// UIConfig serves UI configuration for the dashboard.
type UIConfig struct {
}

// NewUIConfig returns a new UIConfig instance.
func NewUIConfig() *UIConfig {
	return &UIConfig{}
}

// GetHandler returns UI configuration that requires authentication.
func (uic *UIConfig) GetHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, UIConfigResponse{
		WorkatoWebHookURL: configuration.GetRegistrationServiceConfig().WorkatoWebHookURL(),
	})
}

// GetPublicHandler returns UI configuration that does not require authentication.
func (uic *UIConfig) GetPublicHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, UIConfigPublicResponse{
		DisabledIntegrations: configuration.GetRegistrationServiceConfig().DisabledIntegrations(),
	})
}
