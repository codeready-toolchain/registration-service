package controller

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/status"

	"github.com/gin-gonic/gin"
)

type HealthCheckConfig interface {
	GetEnvironment() string
}

// HealthCheck implements the health endpoint.
type HealthCheck struct {
	checker HealthChecker
}

// HealthCheck returns a new HealthCheck instance.
func NewHealthCheck(checker HealthChecker) *HealthCheck {
	return &HealthCheck{
		checker: checker,
	}
}

// getHealthInfo returns the health info.
func (hc *HealthCheck) getHealthInfo() *status.Health {
	cfg := configuration.GetRegistrationServiceConfig()
	return &status.Health{
		Alive:       hc.checker.Alive(),
		Environment: cfg.Environment(),
		Revision:    configuration.Commit,
		BuildTime:   configuration.BuildTime,
		StartTime:   configuration.StartTime,
	}
}

// GetHandler returns a default heath check result.
func (hc *HealthCheck) GetHandler(ctx *gin.Context) {
	// Default handler for system health
	healthInfo := hc.getHealthInfo()
	if healthInfo.Alive {
		ctx.JSON(http.StatusOK, healthInfo)
	} else {
		ctx.JSON(http.StatusServiceUnavailable, healthInfo)
	}
}

type HealthChecker interface {
	Alive() bool
}

func NewHealthChecker() HealthChecker {
	return &healthCheckerImpl{}
}

type healthCheckerImpl struct {
}

func (c *healthCheckerImpl) Alive() bool {
	// TODO check if there are errors in configuration
	return true
}
