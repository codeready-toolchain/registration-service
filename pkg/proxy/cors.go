package proxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/log"
)

const toLower = 'a' - 'A'

// corsPreflightHandler handles the CORS preflight requests
func corsPreflightHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			log.Info(nil, "Handling preflight request")
			handlePreflight(w, r)

			// Preflight requests are standalone and should stop the chain
			w.WriteHeader(http.StatusNoContent)
		} else {
			// Actual request
			h.ServeHTTP(w, r)
		}
	})
}

func handlePreflight(w http.ResponseWriter, r *http.Request) {
	headers := w.Header()
	origin := r.Header.Get("Origin")

	headers.Add("Vary", "Origin")
	headers.Add("Vary", "Access-Control-Request-Method")
	headers.Add("Vary", "Access-Control-Request-Headers")

	// Allow all origins but empty
	if origin == "" {
		log.Info(nil, "Preflight aborted: empty origin")
		return
	}
	// Allow all known methods
	reqMethod := r.Header.Get("Access-Control-Request-Method")
	if !isMethodAllowed(reqMethod) {
		log.Info(nil, fmt.Sprintf("Preflight aborted: method '%s' not allowed", reqMethod))
		return
	}
	// Since we allow all headers we don't check the "Access-Control-Request-Method" header
	reqHeaders := parseHeaderList(r.Header.Get("Access-Control-Request-Headers"))

	// Set the response headers
	headers.Set("Access-Control-Allow-Origin", "*")
	headers.Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
	if len(reqHeaders) > 0 {
		// Simply returning requested headers from Access-Control-Request-Headers should be enough
		headers.Set("Access-Control-Allow-Headers", strings.Join(reqHeaders, ", "))
	}

	// Allow credentials
	headers.Set("Access-Control-Allow-Credentials", "true")
}

var allowedMethods = []string{"PUT", "PATCH", "POST", "GET", "DELETE", "OPTIONS"}

func isMethodAllowed(method string) bool {
	method = strings.ToUpper(method)
	for _, m := range allowedMethods {
		if m == method {
			return true
		}
	}
	return false
}

// addCorsToResponse adds CORS headers to the response
func addCorsToResponse(response *http.Response) error {
	// CORS Headers
	response.Header.Set("Access-Control-Allow-Origin", "*")
	response.Header.Set("Access-Control-Allow-Credentials", "true")
	response.Header.Set("Access-Control-Expose-Headers", "Content-Length, Content-Encoding, Authorization")

	return nil
}

// parseHeaderList tokenize + normalize a string containing a list of headers
func parseHeaderList(headerList string) []string {
	l := len(headerList)
	h := make([]byte, 0, l)
	upper := true
	// Estimate the number headers in order to allocate the right splice size
	t := 0
	for i := 0; i < l; i++ {
		if headerList[i] == ',' {
			t++
		}
	}
	headers := make([]string, 0, t)
	for i := 0; i < l; i++ {
		b := headerList[i]
		switch {
		case b >= 'a' && b <= 'z':
			if upper {
				h = append(h, b-toLower)
			} else {
				h = append(h, b)
			}
		case b >= 'A' && b <= 'Z':
			if !upper {
				h = append(h, b+toLower)
			} else {
				h = append(h, b)
			}
		case b == '-' || b == '_' || b == '.' || (b >= '0' && b <= '9'):
			h = append(h, b)
		}

		if b == ' ' || b == ',' || i == l-1 {
			if len(h) > 0 {
				// Flush the found header
				headers = append(headers, string(h))
				h = h[:0]
				upper = true
			}
		} else {
			upper = b == '-' || b == '_'
		}
	}
	return headers
}
