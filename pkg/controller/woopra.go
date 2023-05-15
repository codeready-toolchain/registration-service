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
func (w *Woopra) GetDevSpacesWoopraDomain(ctx *gin.Context) {
	cfg := configuration.GetRegistrationServiceConfig()
	domain := cfg.Analytics().DevSpacesWoopraDomain()
	ctx.String(http.StatusOK, domain)
}

// GetSandboxSegmentWriteKey returns segment-write-key content for UI.
func (w *Woopra) GetSandboxSegmentWriteKey(ctx *gin.Context) {
	cfg := configuration.GetRegistrationServiceConfig()
	segmentWriteKey := cfg.Analytics().SegmentWriteKey()
	ctx.String(http.StatusOK, segmentWriteKey)
}

// GetDevSpacesSegmentWriteKey returns segment-write-key content for DevSpaces
func (w *Woopra) GetDevSpacesSegmentWriteKey(ctx *gin.Context) {
	cfg := configuration.GetRegistrationServiceConfig()
	segmentWriteKey := cfg.Analytics().DevSpacesSegmentWriteKey()
	ctx.String(http.StatusOK, segmentWriteKey)
}
