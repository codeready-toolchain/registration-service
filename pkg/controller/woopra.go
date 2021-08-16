package controller

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// Woopra implements the segment endpoint, which is invoked to
// retrieve the woopra domain for the ui.
type Woopra struct {
}

// NewWoopra returns a new Woopra instance.
func NewWoopra() *Woopra {
	return &Woopra{}
}

// GetHandler returns the woopra-domain for UI.
func (w *Woopra) GetWoopraDomain(ctx *gin.Context) {
	cfg := configuration.GetCachedRegistrationServiceConfig()
	domain := cfg.Analytics().WoopraDomain()
	ctx.String(http.StatusOK, domain)
}

// GetSegmentWriteKey returns segment-write-key content for UI.
func (w *Woopra) GetSegmentWriteKey(ctx *gin.Context) {
	cfg := configuration.GetCachedRegistrationServiceConfig()
	segmentWriteKey := cfg.Analytics().SegmentWriteKey()
	ctx.String(http.StatusOK, segmentWriteKey)
}
