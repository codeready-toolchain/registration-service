package controller

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

type UIConfigResponse struct {
	// Holds to weight specifying up to how many users ( in percentage ) should use the new UI.
	// NOTE: this is a temporary parameter, it will be removed once we switch all the users to the new UI.
	UICanaryDeploymentWeight int `json:"uiCanaryDeploymentWeight"`
}

// UIConfig implements the ui config endpoint, which is invoked to
// retrieve the config for the ui.
type UIConfig struct {
}

// NewAuthConfig returns a new AuthConfig instance.
func NewUIConfig() *UIConfig {
	return &UIConfig{}
}

// GetHandler returns raw auth config content for UI.
func (uic *UIConfig) GetHandler(ctx *gin.Context) {
	cfg := configuration.GetRegistrationServiceConfig()
	configRespData := UIConfigResponse{
		UICanaryDeploymentWeight: cfg.UICanaryDeploymentWeight(),
	}
	ctx.JSON(http.StatusOK, configRespData)
}
