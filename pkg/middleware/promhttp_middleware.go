package middleware

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
)

// see https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp#example-InstrumentRoundTripperDuration

func InstrumentRoundTripperInFlight(gauge prometheus.Gauge) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			gauge.Inc()
			defer gauge.Dec()
			return next(c)
		}
	}
}

func InstrumentRoundTripperCounter(counter *prometheus.CounterVec) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			counter.With(prometheus.Labels{
				"code":   strconv.Itoa(c.Response().Status),
				"method": c.Request().Method,
				"path":   c.Request().URL.Path,
			}).Inc()
			return err
		}
	}
}

func InstrumentRoundTripperDuration(histVec *prometheus.HistogramVec) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start)
			histVec.With(prometheus.Labels{
				"code":   strconv.Itoa(c.Response().Status),
				"method": c.Request().Method,
				"path":   c.Request().URL.Path,
			}).Observe(float64(duration.Seconds()))
			return err
		}
	}
}
