package registrationserver_test

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/registration-service/pkg/static"
)

func TestRoutes(t *testing.T) {
	// we're using the example config for the configuration here as the
	// specific config params do not matter for testing the routes setup.
	srv, err := registrationserver.New("../../example-config.yml")
	require.NoError(t, err)

	// setting up the routes
	srv.SetupRoutes()

	// setting up the table test
	/*var routetests = []struct {
		path   string
		method string
	}{
		{"/api/v1/health", "GET"},
		{"/", "GET"},
	}*/

	// This doesn't seem to be a particularly useful test - discuss removing it
	/*
		for _, tt := range routetests {
			t.Run(tt.pathTemplate, func(t *testing.T) {
				route := srv.Engine().GetRoute(tt.name)
				pathTemplate, err := route.GetPathTemplate()
				require.NoError(t, err)
				assert.Equal(t, tt.pathTemplate, pathTemplate, "pathTemplate for route '%s' wrong: got %s want %s", tt.name, pathTemplate, tt.pathTemplate)
				pathRegexp, err := route.GetPathRegexp()
				require.NoError(t, err)
				assert.Equal(t, tt.pathRegexp, pathRegexp, "pathRegexp '%s' wrong: got %s want %s", tt.name, pathRegexp, tt.pathRegexp)
				queriesTemplates, err := route.GetQueriesTemplates()
				require.NoError(t, err)
				assert.Equal(t, tt.queriesTemplates, queriesTemplates, "queriesTemplates for route '%s' wrong: got %s want %s", tt.name, queriesTemplates, tt.queriesTemplates)
				queriesRegexps, err := route.GetQueriesRegexp()
				require.NoError(t, err)
				assert.Equal(t, tt.queriesRegexps, queriesRegexps, "queriesRegexps for route '%s' wrong: got %s want %s", tt.name, queriesRegexps, tt.queriesRegexps)
				methods, err := route.GetMethods()
				require.NoError(t, err)
				assert.Equal(t, tt.methods, methods, "methods for route '%s' wrong: got %s want %s", tt.name, methods, tt.methods)
			})
		}*/
}

func TestStaticContent(t *testing.T) {

	// create handler instance.
	spa := registrationserver.SpaHandler{Assets: static.Assets}

	// setting up the table test
	var statictests = []struct {
		name    string
		urlPath string
		fsPath  string
		method  string
		status  int
	}{
		{"Root", "/", "index.html", "GET", http.StatusOK},
		{"Path /index.html", "/index.html", "", "GET", http.StatusOK},
	}
	for _, tt := range statictests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.urlPath, nil)
			require.NoError(t, err)
			// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rr)
			ctx.Request = req
			// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
			// directly and pass in our Request and ResponseRecorder.
			spa.ServeHTTP(ctx)
			// Check the status code is what we expect.
			assert.Equal(t, tt.status, rr.Code, "handler returned wrong status code: got %v want %v", rr.Code, tt.status)
			// Check the response body is what we expect.
			if tt.fsPath != "" {
				buf := bytes.NewBuffer(nil)
				file, err := static.Assets.Open(tt.fsPath)
				require.NoError(t, err)
				io.Copy(buf, file)
				file.Close()
				assert.Equal(t, buf.Bytes(), rr.Body.Bytes(), "handler returned wrong static content: got '%s' want '%s'", string(rr.Body.Bytes()), string(buf.Bytes()))
			} else {
				assert.Equal(t, []byte(nil), rr.Body.Bytes(), "handler returned static content where body should be empty: got '%s'", string(rr.Body.Bytes()))
			}
		})
	}
}
