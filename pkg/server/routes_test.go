package server_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/registration-service/pkg/static"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestRoutesSuite struct {
	testutils.UnitTestSuite
}

func TestRunRoutesSuite(t *testing.T) {
	suite.Run(t, &TestRoutesSuite{testutils.UnitTestSuite{}})
}

func (s *TestRoutesSuite) TestStaticContent() {

	// Create handler instance.
	static := server.StaticHandler{Assets: static.Assets}

	// Setting up the table test
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
		{"Favicon", "/favicon.ico", "/favicon.ico", "", "GET", http.StatusOK},
	}
	for _, tt := range statictests {
		s.Run(tt.name, func() {
			req, err := http.NewRequest(tt.method, tt.urlPath, nil)
			require.NoError(s.T(), err)
			// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rr)
			ctx.Request = req
			// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
			// directly and pass in our Request and ResponseRecorder.
			static.ServeHTTP(ctx)
			// Check the status code is what we expect.
			assert.Equal(s.T(), tt.status, rr.Code, "handler returned wrong status code: got %v want %v", rr.Code, tt.status)
			// Check the response body is what we expect.
			if tt.fsPath != "" {
				buf := bytes.NewBuffer(nil)
				file, err := static.Assets.Open(tt.fsPath)
				require.NoError(s.T(), err)
				io.Copy(buf, file)
				file.Close()
				assert.Equal(s.T(), buf.Bytes(), rr.Body.Bytes(), "handler returned wrong static content: got '%s' want '%s'", string(rr.Body.Bytes()), string(buf.Bytes()))
			} else if tt.expectedContents != "" {
				require.Equal(s.T(), tt.expectedContents, rr.Body.String())
			} else {
				assert.Equal(s.T(), []byte(nil), rr.Body.Bytes(), "handler returned static content where body should be empty: got '%s'", string(rr.Body.Bytes()))
			}
		})
	}
}
