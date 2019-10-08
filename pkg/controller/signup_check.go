package controller

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/gin-gonic/gin"
)

const (
	// SignupStatePending represents a signup in state pending approval.
	SignupStatePending = "pendingApproval"
	// SignupStateProvisioning represents a signup in state in provisioning.
	SignupStateProvisioning = "provisioning"
	// SignupStateProvisioned represents a signup in state provisioned/ready.
	SignupStateProvisioned = "provisioned"
)

// SignupCheck implements the SignupCheck endpoint.
type SignupCheck struct {
	config               *configuration.Registry
	logger               *log.Logger
}

// SignupCheckPayload payload
type SignupCheckPayload struct {
	Ready bool `json:"ready"`
	Reason string `json:"reason"`
	Message string `json:"message"`
}

// NewSignupCheck returns a new SignupCheck instance.
func NewSignupCheck(logger *log.Logger, config *configuration.Registry) *SignupCheck {
	return &SignupCheck{
		logger: logger,
		config: config,
	}
}

var testRequestTimestamp int64

// getTestSignupCheckInfo retrieves a test check info. Used only for tests.
// It reports provisioning/not ready for 5s, then reports state complete.
func (hc *SignupCheck) getTestSignupCheckInfo() *SignupCheckPayload {
	payload := &SignupCheckPayload {
		Ready: true,
		Reason: "",
		Message: "",
	}
	if testRequestTimestamp == 0 {
		testRequestTimestamp = time.Now().Unix()
	}
	if time.Now().Unix()-testRequestTimestamp >= 5 {
		payload.Ready = true
		payload.Reason = SignupStateProvisioned
		payload.Message = "testing mode - done"
	} else {
		payload.Ready = false
		payload.Reason = SignupStateProvisioning
		payload.Message = "testing mode - waiting for timeout"
	}
	return payload
}

// getSignupCheckInfo returns the SignupCheck info.
func (hc *SignupCheck) getSignupCheckInfo() *SignupCheckPayload {
	return &SignupCheckPayload{
		Ready: true,
		Reason: "",
		Message: "",
	}
}

// GetHandler returns a default heath check result.
func (hc *SignupCheck) GetHandler(ctx *gin.Context) {
	// Default handler for system SignupCheck
	ctx.Writer.Header().Set("Content-Type", "application/json")
	var SignupCheckInfo *SignupCheckPayload
	if hc.config.IsTestingMode() {
		SignupCheckInfo = hc.getTestSignupCheckInfo()
	} else {
		SignupCheckInfo = hc.getSignupCheckInfo()
	}
	// the integration with the actual k8s api needs to retrieve the
	// user details from the context here (added by the middleware) and
	// check the provisioning state.
	ctx.Writer.WriteHeader(http.StatusOK)
	err := json.NewEncoder(ctx.Writer).Encode(SignupCheckInfo)
	if err != nil {
		hc.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}
