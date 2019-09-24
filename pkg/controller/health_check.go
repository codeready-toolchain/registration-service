package controller

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/gin-gonic/gin"
)

// HealthCheck implements the health endpoint.
type HealthCheck struct {
	config *configuration.Registry
	logger *log.Logger
}

// Health payload
type Health struct {
	Alive       bool   `json:"alive"`
	TestingMode bool   `json:"testingMode"`
	Revision    string `json:"revision"`
	BuildTime   string `json:"buildTime"`
	StartTime   string `json:"startTime"`
}

// HealthCheck returns a new HealthCheck instance.
func NewHealthCheck(logger *log.Logger, config *configuration.Registry) *HealthCheck {
	return &HealthCheck{
		logger: logger,
		config: config,
	}
}

// getHealthInfo returns the health info.
func (hc *HealthCheck) getHealthInfo() *Health {
	return &Health{
		Alive:       true,
		TestingMode: hc.config.IsTestingMode(),
		Revision:    configuration.Commit,
		BuildTime:   configuration.BuildTime,
		StartTime:   configuration.StartTime,
	}
}

// GetHandler returns a default heath check result.
func (hc *HealthCheck) GetHandler(ctx *gin.Context) {
	// Default handler for system health
	ctx.Writer.Header().Set("Content-Type", "application/json")
	healthInfo := hc.getHealthInfo()
	if healthInfo.Alive {
		ctx.Writer.WriteHeader(http.StatusOK)
	} else {
		ctx.Writer.WriteHeader(http.StatusInternalServerError)
	}
	err := json.NewEncoder(ctx.Writer).Encode(healthInfo)
	if err != nil {
		hc.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}
