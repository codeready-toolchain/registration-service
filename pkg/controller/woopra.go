package controller

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
	"net/http"
)

type woopraResponse struct {
	WoopraDomain string `json:"woopra-domain"`
}

type segmentResponse struct {
	SegmentWriteKey string `json:"segment-write-key"`
}

// Woopra implements the segment endpoint, which is invoked to
// retrieve the woopra domain for the ui.
type Woopra struct {
	config configuration.Configuration
}

// NewWoopra returns a new Woopra instance.
func NewWoopra(config configuration.Configuration) *Woopra {
	return &Woopra{
		config: config,
	}
}

// GetHandler returns the woopra-domain for UI.
func (w *Woopra) GetWoopraDomain(ctx *gin.Context) {
	domain := w.config.GetWoopraDomain()
	ctx.JSON(http.StatusOK, woopraResponse{WoopraDomain: domain})
}

// GetSegmentWriteKey returns segment-write-key content for UI.
func (s *Woopra) GetSegmentWriteKey(ctx *gin.Context) {
	segmentWriteKey := s.config.GetSegmentWriteKey()
	ctx.JSON(http.StatusOK, segmentResponse{SegmentWriteKey: segmentWriteKey})
}