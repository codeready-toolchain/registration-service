package controller

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
	"net/http"
)

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
	ctx.String(http.StatusOK, domain)
}

// GetSegmentWriteKey returns segment-write-key content for UI.
func (s *Woopra) GetSegmentWriteKey(ctx *gin.Context) {
	segmentWriteKey := s.config.GetSegmentWriteKey()
	ctx.String(http.StatusOK, segmentWriteKey)
}
