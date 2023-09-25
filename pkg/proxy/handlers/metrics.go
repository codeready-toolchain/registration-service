package handlers

import (
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct{}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) PrometheusHandler(ctx echo.Context) {
	h := promhttp.HandlerFor(metrics.Reg, promhttp.HandlerOpts{DisableCompression: true, Registry: metrics.Reg})
	h.ServeHTTP(ctx.Response().Writer, ctx.Request())
}
