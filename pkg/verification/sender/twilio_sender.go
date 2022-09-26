package sender

import (
	"fmt"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-gonic/gin"
	"github.com/kevinburke/twilio-go"
)

type twilioNotificationSender struct {
	Config     configuration.RegistrationServiceConfig
	HTTPClient *http.Client
}

func NewTwilioSender(cfg configuration.RegistrationServiceConfig, httpClient *http.Client) NotificationSender {
	return &twilioNotificationSender{
		Config:     cfg,
		HTTPClient: httpClient,
	}
}

func (s *twilioNotificationSender) SendNotification(ctx *gin.Context, content, phoneNumber string) error {

	client := twilio.NewClient(s.Config.Verification().TwilioAccountSID(), s.Config.Verification().TwilioAuthToken(), s.HTTPClient)
	fromNumber := s.Config.Verification().TwilioFromNumber()
	msg, err := client.Messages.SendMessage(fromNumber, phoneNumber, content, nil)
	if err != nil {
		if msg != nil {
			log.Error(ctx, err, fmt.Sprintf("error while sending, code: %d message: %s", msg.ErrorCode, msg.ErrorMessage))
		} else {
			log.Error(ctx, err, "unknown error while sending")
		}

		return err
	}

	return nil
}
