package keycloak

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// KeycloakService implements the keycloak endpoint, which is invoked for new user registrations.
type KeycloakService struct {
	config *configuration.Registry
	logger *log.Logger
}

// NewKeycloakService returns a new KeycloakService instance.
func NewKeycloakService(logger *log.Logger, config *configuration.Registry) *KeycloakService {
	return &KeycloakService{
		logger: logger,
		config: config,
	}
}

// getKeycloakInfo returns the keycloak info.
func (srv *KeycloakService) getKeycloakInfo() map[string]interface{} {
	m := make(map[string]interface{})
	// TODO: this need to get actual keycloak info.
	m["url"] = "http://keycloak-server/auth"
	m["realm"] = "registrationapp"
	m["clientId"] = "registrationapp"
	return m
}

// GetKeycloakHandler returns the keycloak info.
func (srv *KeycloakService) GetKeycloakHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")
	keycloakInfo := srv.getKeycloakInfo()
	if keycloakInfo["clientId"].(string) != "" {
		ctx.Writer.WriteHeader(http.StatusOK)
	} else {
		ctx.Writer.WriteHeader(http.StatusInternalServerError)
	}
	err := json.NewEncoder(ctx.Writer).Encode(keycloakInfo)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
