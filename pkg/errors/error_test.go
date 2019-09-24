package errors_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	err "github.com/codeready-toolchain/registration-service/pkg/errors"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
)

type TestErrorsSuite struct {
	testutils.UnitTestSuite
}

func TestRunErrorsSuite(t *testing.T) {
	suite.Run(t, &TestErrorsSuite{testutils.UnitTestSuite{}})
}

func (s *TestErrorsSuite) TestErrors() {
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)

	s.Run("check json error payload", func() {
		details := "testing payload"
		errMsg := "testing new error"
		code := http.StatusInternalServerError

		err.EncodeError(ctx, errors.New(errMsg), code, details)

		res := err.Error{}
		json.Unmarshal(rr.Body.Bytes(), &res)

		assert.Equal(s.T(), res.Code, http.StatusInternalServerError)
		assert.Equal(s.T(), res.Details, details)
		assert.Equal(s.T(), res.Message, errMsg)
		assert.Equal(s.T(), res.Status, http.StatusText(code))
	})
}
