package controller

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy"
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

type HealthStatus struct {
	*status.Health
	ProxyAlive bool
}

// HealthCheck returns a new HealthCheck instance.
func NewHealthCheck(checker HealthChecker) *HealthCheck {
	return &HealthCheck{
		checker: checker,
	}
}

// getHealthInfo returns the health info.
func (hc *HealthCheck) getHealthInfo(ctx *gin.Context) *HealthStatus {
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
func (hc *HealthCheck) GetHandler(ctx *gin.Context) {
	// Default handler for system health
	healthInfo := hc.getHealthInfo(ctx)
	if healthInfo.Alive {
		ctx.JSON(http.StatusOK, healthInfo)
	} else {
		ctx.JSON(http.StatusServiceUnavailable, healthInfo)
	}
}

type HealthChecker interface {
	Alive(*gin.Context) bool
	APIProxyAlive(*gin.Context) bool
}

func NewHealthChecker() HealthChecker {
	return &healthCheckerImpl{}
}

type healthCheckerImpl struct {
}

func (c *healthCheckerImpl) Alive(ctx *gin.Context) bool {
	// TODO check if there are errors in configuration
	return c.APIProxyAlive(ctx)
}

func (c *healthCheckerImpl) APIProxyAlive(ctx *gin.Context) bool {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/proxyhealth", proxy.ProxyPort))
	if err != nil {
		log.Error(ctx, err, "API Proxy health check failed")
		return false
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(ctx, err, "failed to read API Proxy health check body")
	}
	return resp.StatusCode == http.StatusOK
}
