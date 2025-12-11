package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/assets"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-contrib/static"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticContent(t *testing.T) {
	log.Init("registration-service-testing")
	router := gin.Default()
	staticHandler, err := assets.ServeEmbedContent()
	require.NoError(t, err)
	router.Use(static.Serve("/", staticHandler))

	router.RedirectTrailingSlash = true

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	router.ServeHTTP(w, req)

	// Setting up the table test
	var statictests = []struct {
		requestPath    string
		requestMethod  string
		assertResponse assertResponse
	}{
		{
			requestMethod:  "GET",
			requestPath:    "/",
			assertResponse: okWithBodyFromEmbedFS("static/index.html"),
		},
		{
			requestMethod:  "GET",
			requestPath:    "/index.html",
			assertResponse: movedTo("./"),
		},
		{
			requestMethod:  "GET",
			requestPath:    "/nonexistent",
			assertResponse: notFound(),
		},
		{
			requestMethod:  "GET",
			requestPath:    "/favicon.ico",
			assertResponse: okWithBodyFromEmbedFS("static/favicon.ico"),
		},

		// {"Path /index.html", "static/index.html", "", "", "GET", http.StatusOK},
		// {"Path /nonexistent", "/nonexistent", "", "<a href=\"/index.html\">See Other</a>.\n\n", "GET", http.StatusSeeOther},
		// {"Favicon", "/favicon.ico", "/favicon.ico", "", "GET", http.StatusOK},
	}
	for _, tt := range statictests {
		t.Run(fmt.Sprintf("%s %s", tt.requestMethod, tt.requestPath), func(t *testing.T) {
			req, err := http.NewRequest(tt.requestMethod, tt.requestPath, nil)
			require.NoError(t, err)
			// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr := httptest.NewRecorder()
			// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
			// directly and pass in our Request and ResponseRecorder.
			router.ServeHTTP(rr, req)
			// Check the response
			tt.assertResponse(t, rr)
		})
	}
}

type assertResponse func(t *testing.T, rr *httptest.ResponseRecorder)

func okWithBodyFromEmbedFS(path string) assertResponse {
	return func(t *testing.T, rr *httptest.ResponseRecorder) {
		require.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
		expectedContent, err := assets.StaticContent.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, string(expectedContent), rr.Body.String(), "handler returned wrong static content")
	}
}

func movedTo(path string) assertResponse {
	return func(t *testing.T, rr *httptest.ResponseRecorder) {
		require.Equal(t, http.StatusMovedPermanently, rr.Code, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusTemporaryRedirect)
		assert.Equal(t, []string{path}, rr.Header()["Location"])
	}
}

func notFound() assertResponse {
	return func(t *testing.T, rr *httptest.ResponseRecorder) {
		require.Equal(t, http.StatusNotFound, rr.Code, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusNotFound)
	}
}
