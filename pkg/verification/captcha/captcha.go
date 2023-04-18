package captcha

import (
	gocontext "context"
	"fmt"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-gonic/gin"

	recaptcha "cloud.google.com/go/recaptchaenterprise/v2/apiv1"
	recaptchapb "cloud.google.com/go/recaptchaenterprise/v2/apiv1/recaptchaenterprisepb"
)

// recaptchaSignupAction is the action name corresponding to the token
const recaptchaSignupAction = "SIGNUP"

type Assessor interface {
	CompleteAssessment(ctx *gin.Context, cfg configuration.RegistrationServiceConfig, token string) (float32, error)
}

type Helper struct{}

/*
*
* Creates an assessment to analyze the risk of a signup.
*
* @param ctx: The request context.
* @param cfg: The Registration Service Configuration object.
* @param token: The token obtained from the client on passing the reCAPTCHA Site Key.

returns an error if the assessment failed due to error or the assessment score was below the threshold.
*/
func (c Helper) CompleteAssessment(ctx *gin.Context, cfg configuration.RegistrationServiceConfig, token string) (float32, error) {
	gctx := gocontext.Background()
	client, err := recaptcha.NewClient(gctx)
	if err != nil {
		return -1, fmt.Errorf("error creating reCAPTCHA client")
	}
	defer client.Close()

	// Set the properties of the event to be tracked.
	event := &recaptchapb.Event{
		Token:   token,
		SiteKey: cfg.Verification().CaptchaSiteKey(),
	}

	assessment := &recaptchapb.Assessment{
		Event: event,
	}

	// Build the assessment request.
	request := &recaptchapb.CreateAssessmentRequest{
		Assessment: assessment,
		Parent:     fmt.Sprintf("projects/%s", cfg.Verification().CaptchaProjectID()),
	}

	response, err := client.CreateAssessment(
		ctx,
		request)
	if err != nil {
		return -1, fmt.Errorf("failed to create reCAPTCHA assessment")
	}

	// Check if the token is valid.
	if !response.TokenProperties.Valid {
		return -1, fmt.Errorf("the CreateAssessment() call failed because the token"+
			" was invalid for the following reasons: %v",
			response.TokenProperties.InvalidReason)
	}

	// Check if the expected action was executed.
	if response.TokenProperties.Action == recaptchaSignupAction {
		// Get the risk score and the reason(s).
		// For more information on interpreting the assessment,
		// see: https://cloud.google.com/recaptcha-enterprise/docs/interpret-assessment
		log.Info(ctx, fmt.Sprintf("The reCAPTCHA assessment score is:  %v", response.RiskAnalysis.Score))

		for _, reason := range response.RiskAnalysis.Reasons {
			log.Info(ctx, fmt.Sprintf("Risk analysis reason: %s", reason.String()))
		}
		return response.RiskAnalysis.Score, nil
	}

	return -1, fmt.Errorf("the action attribute in the reCAPTCHA token does not match the expected action to score")
}
