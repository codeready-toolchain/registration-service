package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// see https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp#example-InstrumentRoundTripperDuration

func InstrumentRoundTripperInFlight(gauge prometheus.Gauge) gin.HandlerFunc {
	return func(c *gin.Context) {
		gauge.Inc()
		defer func() {
			gauge.Dec()
		}()
		c.Next()
	}
}

func InstrumentRoundTripperCounter(counter *prometheus.CounterVec) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			counter.With(prometheus.Labels{
				"code":   strconv.Itoa(c.Writer.Status()),
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
			}).Inc()
		}()
		c.Next()
	}
}

func InstrumentRoundTripperDuration(histVec *prometheus.HistogramVec) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		defer func() {
			duration := time.Since(start)
			histVec.With(prometheus.Labels{
				"code":   strconv.Itoa(c.Writer.Status()),
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
			}).Observe(float64(duration.Milliseconds()))
		}()
		c.Next()
	}
}
