package controller

import (
	"fmt"
	"io"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/toolchain-common/pkg/status"

	"github.com/labstack/echo/v4"
)

type HealthCheckConfig interface {
	GetEnvironment() string
}

// HealthCheck implements the health endpoint.
type HealthCheck struct {
	checker HealthChecker
}

type HealthStatus struct {
	*status.Health
	ProxyAlive bool `json:"proxyAlive"`
}

// HealthCheck returns a new HealthCheck instance.
func NewHealthCheck(checker HealthChecker) *HealthCheck {
	return &HealthCheck{
		checker: checker,
	}
}

// getHealthInfo returns the health info.
func (hc *HealthCheck) getHealthInfo(ctx echo.Context) *HealthStatus {
	cfg := configuration.GetRegistrationServiceConfig()
	return &HealthStatus{
		Health: &status.Health{
			Alive:       hc.checker.Alive(ctx),
			Environment: cfg.Environment(),
			Revision:    configuration.Commit,
			BuildTime:   configuration.BuildTime,
			StartTime:   configuration.StartTime,
		},
		ProxyAlive: hc.checker.APIProxyAlive(ctx),
	}
}

// GetHandler returns a default heath check result.
func (hc *HealthCheck) GetHandler(ctx echo.Context) error {
	healthInfo := hc.getHealthInfo(ctx)
	if healthInfo.Alive {
		return ctx.JSON(http.StatusOK, healthInfo)
	}
	return ctx.JSON(http.StatusServiceUnavailable, healthInfo)
}

type HealthChecker interface {
	Alive(echo.Context) bool
	APIProxyAlive(echo.Context) bool
}

func NewHealthChecker(port string) HealthChecker {
	return &healthCheckerImpl{port: port}
}

type healthCheckerImpl struct {
	port string
}

func (c *healthCheckerImpl) Alive(ctx echo.Context) bool {
	// TODO check if there are errors in configuration
	return c.APIProxyAlive(ctx)
}

func (c *healthCheckerImpl) APIProxyAlive(ctx echo.Context) bool {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/proxyhealth", c.port))
	if err != nil {
		log.Error(ctx, err, "API Proxy health check failed")
		return false
	}
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Error(ctx, err, "failed to read API Proxy health check body")
	}
	return resp.StatusCode == http.StatusOK
}
