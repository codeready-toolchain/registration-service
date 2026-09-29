package controller

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

type UIConfigResponse struct {
	DisabledIntegrations []string `json:"disabledIntegrations"`
	WorkatoWebHookURL    string   `json:"workatoWebHookURL"`
}

type UIConfigPublicResponse struct {
	DisabledIntegrations []string `json:"disabledIntegrations"`
}

// UIConfig implements the ui config endpoint, which is invoked to
// retrieve the config for the ui.
type UIConfig struct {
}

// NewUIConfig returns a new UIConfig instance.
func NewUIConfig() *UIConfig {
	return &UIConfig{}
}

// GetHandler returns the UI config content for authenticated users.
func (uic *UIConfig) GetHandler(ctx *gin.Context) {
	cfg := configuration.GetRegistrationServiceConfig()
	configRespData := UIConfigResponse{
		DisabledIntegrations: cfg.DisabledIntegrations(),
		WorkatoWebHookURL:    cfg.WorkatoWebHookURL(),
	}
	ctx.JSON(http.StatusOK, configRespData)
}

// GetPublicHandler returns the UI config content that can be accessed
// without authentication. This is useful for the landing page to know
// which integrations are disabled.
func (uic *UIConfig) GetPublicHandler(ctx *gin.Context) {
	configRespData := UIConfigPublicResponse{
		DisabledIntegrations: configuration.GetRegistrationServiceConfig().DisabledIntegrations(),
	}
	ctx.JSON(http.StatusOK, configRespData)
}
