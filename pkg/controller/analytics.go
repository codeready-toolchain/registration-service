package controller

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// Analytics implements the segment endpoint, which is invoked to
// retrieve the amplitude domain for the ui.
type Analytics struct {
}

// NewAnalytics returns a new Analytics instance.
func NewAnalytics() *Analytics {
	return &Analytics{}
}

// GetSandboxSegmentWriteKey returns segment-write-key content for UI.
func (a *Analytics) GetSandboxSegmentWriteKey(ctx *gin.Context) {
	cfg := configuration.GetRegistrationServiceConfig()
	segmentWriteKey := cfg.Analytics().SegmentWriteKey()
	ctx.String(http.StatusOK, segmentWriteKey)
}

// GetDevSpacesSegmentWriteKey returns segment-write-key content for DevSpaces
func (a *Analytics) GetDevSpacesSegmentWriteKey(ctx *gin.Context) {
	cfg := configuration.GetRegistrationServiceConfig()
	segmentWriteKey := cfg.Analytics().DevSpacesSegmentWriteKey()
	ctx.String(http.StatusOK, segmentWriteKey)
}
