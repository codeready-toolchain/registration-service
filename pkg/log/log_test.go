package log_test

import (
	"bytes"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/log"
	testutils "github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestLogSuite struct {
	testutils.UnitTestSuite
}

func TestRunLogSuite(t *testing.T) {
	suite.Run(t, &TestLogSuite{testutils.UnitTestSuite{}})
}

func (s *TestLogSuite) TestLogHandler() {
	log.InitializeLogger(os.Stdout, "testing: ", 0)

	s.Run("test flags", func() {
		assert.Equal(s.T(), log.Flags(), 0)
	})

	s.Run("test prefix", func() {
		assert.Equal(s.T(), log.Prefix(), "testing: ")
	})

	s.Run("test println", func() {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer func() {
			log.SetOutput(os.Stderr)
		}()

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		log.Println(ctx, "println")
		assert.Equal(s.T(), buf.String(), "testing: [[println] context subject: test]\n")
	})

	s.Run("test print", func() {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer func() {
			log.SetOutput(os.Stderr)
		}()

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		log.Print(ctx, "print")
		assert.Equal(s.T(), buf.String(), "testing: [[print] context subject: test]\n")
	})

	s.Run("test printf", func() {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer func() {
			log.SetOutput(os.Stderr)
		}()

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		log.Printf(ctx, "%s", "printf")
		assert.Equal(s.T(), buf.String(), "testing: [[printf] context subject: test]\n")
	})
}
