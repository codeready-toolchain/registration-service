package controller

import (
	"encoding/json"
	"log"
	"net/http"

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
	config      *configuration.Registry
	logger      *log.Logger
	checkerFunc func(ctx *gin.Context) *SignupCheckPayload
}

// SignupCheckPayload payload
type SignupCheckPayload struct {
	Ready   bool   `json:"ready"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// NewSignupCheck returns a new SignupCheck instance. The checker is the
// func being called when retrieving the provisioning state. It defaults
// to the implementation of getSignupCheckInfo in SignupCheck. Giving
// a custom checker is usually used for testing.
func NewSignupCheck(logger *log.Logger, config *configuration.Registry, checker func(ctx *gin.Context) *SignupCheckPayload) *SignupCheck {
	sc := &SignupCheck{
		logger: logger,
		config: config,
	}
	if checker != nil {
		sc.checkerFunc = checker
	} else {
		sc.checkerFunc = sc.getSignupCheckInfo
	}
	return sc
}

// getSignupCheckInfo returns the SignupCheck info.
func (hc *SignupCheck) getSignupCheckInfo(ctx *gin.Context) *SignupCheckPayload {
	// the integration with the actual k8s api needs to retrieve the
	// user details from the context here (added by the middleware) and
	// check the provisioning state.
	return &SignupCheckPayload{
		Ready:   true,
		Reason:  "",
		Message: "",
	}
}

// GetHandler returns a default heath check result.
func (hc *SignupCheck) GetHandler(ctx *gin.Context) {
	// Default handler for system SignupCheck
	ctx.Writer.Header().Set("Content-Type", "application/json")
	SignupCheckInfo := hc.checkerFunc(ctx)
	ctx.Writer.WriteHeader(http.StatusOK)
	err := json.NewEncoder(ctx.Writer).Encode(SignupCheckInfo)
	if err != nil {
		hc.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}
