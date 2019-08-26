package health

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

// Service implements the service health endpoint.
type Service struct {
	config *configuration.Registry
	logger *log.Logger
}

// New returns a new healthService instance.
func New(logger *log.Logger, config *configuration.Registry) *Service {
	r := new(Service)
	r.logger = logger
	r.config = config
	return r
}

// getHealthInfo returns the health info.
func (srv *Service) getHealthInfo() map[string]interface{} {
	m := make(map[string]interface{})
	// TODO: this need to get actual health info.
	m["alive"] = !srv.config.IsTestingMode()
	m["testingmode"] = srv.config.IsTestingMode()
	m["version"] = srv.config.GetVersion()
	return m
}

// HealthCheckHandler returns a default heath check result.
func (srv *Service) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// default handler for system health
	w.Header().Set("Content-Type", "application/json")
	healthInfo := srv.getHealthInfo()
	if healthInfo["alive"].(bool) {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	err := json.NewEncoder(w).Encode(healthInfo)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}
