package middleware

import (
	"net/http"
	"strconv"
	"time"

	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"

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
				"code":   strconv.Itoa(responseStatus(c, err)),
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
				"code":   strconv.Itoa(responseStatus(c, err)),
				"method": c.Request().Method,
				"path":   c.Request().URL.Path,
			}).Observe(float64(duration.Seconds()))
			return err
		}
	}
}

// responseStatus returns the HTTP status code to use for metrics.
// When the handler returned an error, the status is extracted from the error
// because Echo's centralized error handler hasn't written the response yet at
// this point in the middleware chain, so c.Response().Status may still be 0.
func responseStatus(c echo.Context, err error) int {
	if err != nil {
		return crterrors.StatusCode(err)
	}
	status := c.Response().Status
	if status == 0 {
		return http.StatusOK
	}
	return status
}
