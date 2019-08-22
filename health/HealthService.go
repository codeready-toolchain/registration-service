package health

import (
	"encoding/json"
	"net/http"
)

// Service implements the service health endpoint.
type Service struct {
	isTestMode bool
}

// NewHealthService returns a new HealthService instance.
func NewHealthService(isTestMode bool) *Service {
	r := new(Service)
	r.isTestMode = isTestMode
	return r
}

// getHealthInfo returns the health info.
func (s *Service) getHealthInfo() map[string]bool {
	m := make(map[string]bool)
	m["alive"] = true
	m["testmode"] = s.isTestMode
	return m
}

// Handler is the request handler function and will be called with incoming requests.
func (s *Service) Handler(w http.ResponseWriter, r *http.Request) {
	// default handler for system health
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.getHealthInfo())
}
