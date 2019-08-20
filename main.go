//go:generate go run -tags=dev static/assets_generate.go

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/codeready-toolchain/registration-service/static"

	"github.com/gorilla/mux"
)

// spaHandler implements the http.Handler interface, so we can use it
// to respond to HTTP requests. The path to the static directory and
// path to the index file within that static directory are used to
// serve the SPA in the given static directory.
type spaHandler struct {
	Assets http.FileSystem
}

// ServeHTTP inspects the URL path to locate a file within the static dir
// on the SPA handler. If a file is found, it will be served. If not, the
// file located at the index path on the SPA handler will be served. This
// is suitable behavior for serving an SPA (single page application).
func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// get the absolute path to prevent directory traversal
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		// no absolute path, respond with a 400 bad request and stop
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// check if the file exists in the assets
	_, err = h.Assets.Open(path)
	if err != nil {
		// file does not exist, redirect to index
		log.Printf("File %s does not exist.", path)
		http.Redirect(w, r, "/index.html", http.StatusSeeOther)
		return
	}

	// otherwise, use http.FileServer to serve the static dir
	http.FileServer(h.Assets).ServeHTTP(w, r)
}

// HealthCheckHandler returns a default heath check result.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// default handler for system health
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"alive": true})
}

func main() {
	// create new Gorilla router
	router := mux.NewRouter()

	// create the routes for the api endpoints
	router.HandleFunc("/api/health", HealthCheckHandler)

	// create the route for static content, served from /
	spa := spaHandler{Assets: static.Assets}
	router.PathPrefix("/").Handler(spa)

	// finally, create and start the service
	srv := &http.Server{
		Handler:      router,
		Addr:         "127.0.0.1:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
