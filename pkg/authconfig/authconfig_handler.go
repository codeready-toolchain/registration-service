package authconfig

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// Service implements the auth config endpoint, which is invoked to 
// retrieve the auth config for the ui.
type Service struct {
	config *configuration.Registry
	logger *log.Logger
}

// New returns a new Service instance.
func New(logger *log.Logger, config *configuration.Registry) *Service {
	return &Service{
		logger: logger,
		config: config,
	}
}

func (srv *Service) getAuthClientConfig() map[string]interface{} {
	m := make(map[string]interface{})
	m["realm"] = srv.config.GetAuthClientConfigRealm()
	m["auth-server-url"] = srv.config.GetAuthClientConfigAuthServerURL()
	m["ssl-required"] = srv.config.GetAuthClientConfigSSLRequired()
	m["resource"] = srv.config.GetAuthClientConfigResource()
	m["public-client"] = srv.config.IsAuthClientConfigPublicClient()
	m["confidential-port"] = srv.config.GetAuthClientConfigConfidentialPort()
	return m
}

// AuthconfigHandler returns signup info.
func (srv *Service) AuthconfigHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")
	ctx.Writer.WriteHeader(http.StatusOK)
	authClientConfig := srv.getAuthClientConfig()
	err := json.NewEncoder(ctx.Writer).Encode(authClientConfig)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
