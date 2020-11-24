package controller

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
	"net/http"
)

type segmentResponse struct {
	SegmentWriteKey string `json:"segment-write-key"`
}

// Segment implements the segment endpoint, which is invoked to
// retrieve the segment-write-key for the ui.
type Segment struct {
	config configuration.Configuration
}

// NewSegment returns a new Segment instance.
func NewSegment(config configuration.Configuration) *Segment {
	return &Segment{
		config: config,
	}
}

// GetSegmentWriteKey returns segment-write-key content for UI.
func (s *Segment) GetSegmentWriteKey(ctx *gin.Context) {
	segmentWriteKey := s.config.GetSegmentWriteKey()
	ctx.JSON(http.StatusOK, segmentResponse{SegmentWriteKey: segmentWriteKey})
}
