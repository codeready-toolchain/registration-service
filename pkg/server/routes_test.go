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

func TestStaticContent(t *testing.T) {

	// create handler instance.
	spa := registrationserver.SpaHandler{Assets: static.Assets}

	// setting up the table test
	var statictests = []struct {
		name             string
		urlPath          string
		fsPath           string
		expectedContents string
		method           string
		status           int
	}{
		{"Root", "/", "index.html", "", "GET", http.StatusOK},
		{"Path /index.html", "/index.html", "", "", "GET", http.StatusOK},
		{"Path /nonexistent", "/nonexistent", "", "<a href=\"/index.html\">See Other</a>.\n\n", "GET", http.StatusSeeOther},
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
			} else if tt.expectedContents != "" {
				require.Equal(t, tt.expectedContents, rr.Body.String())
			} else {
				assert.Equal(t, []byte(nil), rr.Body.Bytes(), "handler returned static content where body should be empty: got '%s'", string(rr.Body.Bytes()))
			}
		})
	}
}
