package controller

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// Segment implements the segment endpoint, which is invoked to
// retrieve the segment-write-key for the ui.
type Segment struct {
	config configuration.Configuration
}

// NewSegment returns a new Segment instance.
func NewSegment() *Segment {
	return &Segment{
	}
}

// GetSegmentWriteKey returns segment-write-key content for UI.
func (s *Segment) GetSegmentWriteKey(ctx *gin.Context) {

}
