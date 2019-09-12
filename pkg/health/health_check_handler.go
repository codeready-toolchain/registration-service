package health

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// HealthCheckService implements the service health endpoint.
type HealthCheckService struct {
	config *configuration.Registry
	logger *log.Logger
}

// NewHealthCheckService returns a new HealthCheckService instance.
func NewHealthCheckService(logger *log.Logger, config *configuration.Registry) *HealthCheckService {
	return &HealthCheckService{
		logger: logger,
		config: config,
	}
}

// getHealthInfo returns the health info.
func (srv *HealthCheckService) getHealthInfo() map[string]interface{} {
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
func (srv *HealthCheckService) GetHealthCheckHandler(ctx *gin.Context) {
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
