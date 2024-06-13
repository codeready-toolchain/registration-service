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
	CompleteAssessment(ctx *gin.Context, cfg configuration.RegistrationServiceConfig, token string) (*recaptchapb.Assessment, error)
}

type Helper struct{}

/*
*
* Creates an assessment to analyze the risk of a signup.
*
* @param ctx: The request context.
* @param cfg: The Registration Service Configuration object.
* @param token: The token obtained from the client on passing the reCAPTCHA Site Key.

returns the assessment and nil if the assessment was successful, otherwise returns nil and the error.
*/
func (c Helper) CompleteAssessment(ctx *gin.Context, cfg configuration.RegistrationServiceConfig, token string) (*recaptchapb.Assessment, error) {
	gctx := gocontext.Background()
	client, err := recaptcha.NewClient(gctx)
	if err != nil {
		return nil, fmt.Errorf("error creating reCAPTCHA client")
	}
	defer client.Close()

	// Set the properties of the event to be tracked.
	event := &recaptchapb.Event{
		ExpectedAction: recaptchaSignupAction,
		Token:          token,
		SiteKey:        cfg.Verification().CaptchaSiteKey(),
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
		return nil, fmt.Errorf("failed to create reCAPTCHA assessment")
	}

	// Check if the token is valid.
	if !response.GetTokenProperties().GetValid() {
		return nil, fmt.Errorf("the CreateAssessment() call failed because the token"+
			" was invalid for the following reasons: %v",
			response.GetTokenProperties().GetInvalidReason())
	}

	// Check if the expected action was executed.
	if response.GetTokenProperties().GetAction() == recaptchaSignupAction {
		// Get the risk score and the reason(s).
		// For more information on interpreting the assessment,
		// see: https://cloud.google.com/recaptcha-enterprise/docs/interpret-assessment
		log.Info(ctx, fmt.Sprintf("reCAPTCHA assessment score: %.1f", response.GetRiskAnalysis().GetScore()))

		for _, reason := range response.GetRiskAnalysis().GetReasons() {
			log.Info(ctx, fmt.Sprintf("Risk analysis reason: %s", reason.String()))
		}
		log.Info(ctx, fmt.Sprintf("Assessment Response: %+v", response))
		return response, nil
	}

	return nil, fmt.Errorf("the action attribute in the reCAPTCHA token does not match the expected action to score")
}
