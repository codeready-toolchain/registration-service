//go:generate go run -tags=dev static/assets_generate.go

package main

import (
	"encoding/json"
	"flag"
	"log"
	//	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

	// create the command line flags
	isInsecure := flag.Bool("insecure", false, "service should run as a http service.")
	certPath := flag.String("cert", "", "path to ssl certificate.")
	keyPath := flag.String("key", "", "path to ssl key.")
	port := flag.Int("port", -1, "use port for service")
	flag.Parse()

	// some sanity checks
	if !*isInsecure {
		if *certPath == "" || *keyPath == "" {
			log.Fatal("when running in https mode, certificate and key path needs to be given")
		}
		if _, err := os.Stat(*certPath); os.IsNotExist(err) {
			log.Fatalf("given certificate file does not exist: '%s'.", *certPath)
		}
		if _, err := os.Stat(*keyPath); os.IsNotExist(err) {
			log.Fatalf("given key file does not exist: '%s'.", *keyPath)
		}
	}

	// create new Gorilla router
	router := mux.NewRouter()

	// create the routes for the api endpoints
	router.HandleFunc("/api/health", HealthCheckHandler)

	// create the route for static content, served from /
	spa := spaHandler{Assets: static.Assets}
	router.PathPrefix("/").Handler(spa)

	// assign default ports
	if *port == -1 && *isInsecure {
		*port = 80
	} else if *port == -1 {
		*port = 443
	}

	// some initial log output
	log.Printf("registration service starting on port %d", *port)
	if *isInsecure {
		log.Println("running in insecure mode, http only.")
	} else {
		log.Println("running in secure mode, https only.")
	}

	// finally, create and start the service
	srv := &http.Server{
		Handler:      router,
		Addr:         "0.0.0.0:" + strconv.Itoa(*port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	if *isInsecure {
		log.Fatal(srv.ListenAndServe())
	}
	log.Fatal(srv.ListenAndServeTLS(*certPath, *keyPath))
}
