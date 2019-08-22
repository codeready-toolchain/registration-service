package registrationserver

import (
	"encoding/json"
	"net/http"
)

// HealthCheckHandler returns a default heath check result.
func (srv *RegistrationServer) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// default handler for system health
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"alive": true})
}
