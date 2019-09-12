package health

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// Service implements the service health endpoint.
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

// getHealthInfo returns the health info.
func (srv *Service) getHealthInfo() map[string]interface{} {
	m := make(map[string]interface{})
	// TODO: this need to get actual health info.
	m["alive"] = !srv.config.IsTestingMode()
	m["testingmode"] = srv.config.IsTestingMode()
	m["revision"] = configuration.Commit
	m["build_time"] = configuration.BuildTime
	m["start_time"] = configuration.StartTime
	return m
}

// HealthCheckHandler returns a default heath check result.
func (srv *Service) HealthCheckHandler(ctx *gin.Context) {
	// Default handler for system health
	ctx.Writer.Header().Set("Content-Type", "application/json")
	healthInfo := srv.getHealthInfo()
	if healthInfo["alive"].(bool) {
		ctx.Writer.WriteHeader(http.StatusOK)
	} else {
		ctx.Writer.WriteHeader(http.StatusInternalServerError)
	}
	err := json.NewEncoder(ctx.Writer).Encode(healthInfo)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
