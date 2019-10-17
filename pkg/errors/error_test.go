package errors_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	err "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
)

type TestErrorsSuite struct {
	test.UnitTestSuite
}

func TestRunErrorsSuite(t *testing.T) {
	suite.Run(t, &TestErrorsSuite{test.UnitTestSuite{}})
}

func (s *TestErrorsSuite) TestErrors() {
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)

	s.Run("check json error payload", func() {
		details := "testing payload"
		errMsg := "testing new error"
		code := http.StatusInternalServerError

		err.AbortWithError(ctx, code, errors.New(errMsg), details)

		res := err.Error{}
		err := json.Unmarshal(rr.Body.Bytes(), &res)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), res.Code, http.StatusInternalServerError)
		assert.Equal(s.T(), res.Details, details)
		assert.Equal(s.T(), res.Message, errMsg)
		assert.Equal(s.T(), res.Status, http.StatusText(code))
	})
}
