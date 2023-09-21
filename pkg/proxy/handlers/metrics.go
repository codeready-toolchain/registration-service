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

func (m *Metrics) PrometheusHandler(ctx echo.Context) error {
	h := promhttp.HandlerFor(metrics.Reg, promhttp.HandlerOpts{DisableCompression: true, Registry: metrics.Reg})
	ctx.Response().Writer.Header().Set("Content-Type", "text/plain")
	//c.Writer.Header().Set("content-Type", "text/plain")
	//c.Request.Header.Set("content-Type", "text/plain")
	h.ServeHTTP(ctx.Response().Writer, ctx.Request())
	return nil
}