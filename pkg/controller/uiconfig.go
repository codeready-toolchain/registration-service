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
	// TODO: DisabledIntegrations is also served by /uiconfig/public.
	// It remains here so the current dashboard, which reads both fields from /uiconfig, keeps working.
	// Remove it from this response once the dashboard reads it from /uiconfig/public.
	DisabledIntegrations []string `json:"disabledIntegrations"`
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
	cfg := configuration.GetRegistrationServiceConfig()
	ctx.JSON(http.StatusOK, UIConfigResponse{
		WorkatoWebHookURL:    cfg.WorkatoWebHookURL(),
		DisabledIntegrations: cfg.DisabledIntegrations(),
	})
}

// GetPublicHandler returns UI configuration that does not require authentication.
func (uic *UIConfig) GetPublicHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, UIConfigPublicResponse{
		DisabledIntegrations: configuration.GetRegistrationServiceConfig().DisabledIntegrations(),
	})
}
