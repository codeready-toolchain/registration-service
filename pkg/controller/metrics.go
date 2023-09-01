package controller

import (
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct{}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) PrometheusHandler(c *gin.Context) {
	h := promhttp.HandlerFor(metrics.Reg, promhttp.HandlerOpts{Registry: metrics.Reg})
	c.Writer.Header().Set("content-Type", "text/plain")
	h.ServeHTTP(c.Writer, c.Request)
}
