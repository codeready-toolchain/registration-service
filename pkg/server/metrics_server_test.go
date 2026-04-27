package server_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestStartMetricsServer(t *testing.T) {
	// given

	configuration.Commit = "1234567890"
	registry := prometheus.NewRegistry()
	configuration.RegisterVersionMetrics(registry)
	srv, _ := server.StartMetricsServer(registry, 8080)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		err := srv.Shutdown(ctx)
		if err != nil {
			t.Fatalf("failed to shutdown metrics server: %v", err)
		}
	}(ctx)
	// make a request to the metrics server

	resp, err := http.Get("http://localhost:8080/metrics")
	if err != nil {
		t.Fatalf("failed to make request to metrics server: %v", err)
	}
	defer resp.Body.Close()

	// check the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	// check the response body
	assert.Contains(t, string(body), "sandbox_registration_service_commit")
	assert.Contains(t, string(body), "sandbox_registration_service_short_commit")
}
