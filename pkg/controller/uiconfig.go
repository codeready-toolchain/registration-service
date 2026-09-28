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

	WorkatoWebHookURL string `json:"workatoWebHookURL"`

	DisabledIntegrations []string `json:"disabledIntegrations"`
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
//
// Deprecated: the "GetDisabledIntegrations" and "GetWorkatoWebhookURL" are
// the ones to use to replace this function, which will be removed once these
// changes are deployed in production.
//
// The reason for this is that we want to have a landing page in
// the Sandbox, and the "DisabledIntegrations" information needs to be fetched
// in an unauthenticated manner, so that we can hide certain integrations in
// the landing page too.
//
// The UICanaryDeploymentWeight is not used at all, and the WorkatoWebHookURL
// still needs to be sent only when the user is authenticated, so it makes
// sense to put the latter in its own endpoint similar to what we do with the
// Segment keys.
func (uic *UIConfig) GetHandler(ctx *gin.Context) {
	cfg := configuration.GetRegistrationServiceConfig()
	configRespData := UIConfigResponse{
		UICanaryDeploymentWeight: cfg.UICanaryDeploymentWeight(),
		WorkatoWebHookURL:        cfg.WorkatoWebHookURL(),
		DisabledIntegrations:     cfg.DisabledIntegrations(),
	}
	ctx.JSON(http.StatusOK, configRespData)
}

// GetDisabledIntegrations returns a list of integrations that are currently
// disabled and should not show in the UI. An empty array is returned if the
// list is empty.
func (uic *UIConfig) GetDisabledIntegrations(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, configuration.GetRegistrationServiceConfig().DisabledIntegrations())
}
