package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestSegmentSuite struct {
	test.UnitTestSuite
}

func TestRunSegmentSuite(t *testing.T) {
	suite.Run(t, &TestSegmentSuite{test.UnitTestSuite{}})
}

func (s *TestSegmentSuite) TestSegmentHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/segment-write-key", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config().IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.

	segmentCtrl := NewSegment(s.Config())
	handler := gin.HandlerFunc(segmentCtrl.GetSegmentWriteKey)

	s.Run("valid segment write key json", func() {

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		s.ViperConfig().GetViperInstance().Set("segment.write_key", "testing segment write key")
		assert.Equal(s.T(), "testing segment write key", s.Config().GetSegmentWriteKey())

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		// Check the response body is what we expect.
		// get config values from endpoint response
		var dataEnvelope *segmentResponse
		err = json.Unmarshal(rr.Body.Bytes(), &dataEnvelope)
		require.NoError(s.T(), err)

		s.Run("envelope segment write key", func() {
			assert.Equal(s.T(), s.Config().GetSegmentWriteKey(), dataEnvelope.SegmentWriteKey, "wrong 'segment write key' in segment response")
		})
	})
}
