package sender

import (
	"fmt"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-gonic/gin"
	"github.com/kevinburke/twilio-go"
)

type TwilioNotificationSender struct {
	Config     configuration.RegistrationServiceConfig
	HTTPClient *http.Client

	//SenderIDs is a map containing country codes (key) and associated sender id (value)
	SenderIDs map[string]string
}

func NewTwilioSender(cfg configuration.RegistrationServiceConfig, httpClient *http.Client) NotificationSender {
	sender := &TwilioNotificationSender{
		Config:     cfg,
		HTTPClient: httpClient,
	}

	// Initialize the SenderIDs map
	sender.SenderIDs = make(map[string]string)

	// Populate the SenderIDs map with the configured sender IDs
	for _, senderConfig := range cfg.Verification().TwilioSenderConfigs() {
		for _, countryCode := range senderConfig.CountryCodes {
			sender.SenderIDs[countryCode] = senderConfig.SenderID
		}
	}

	return sender
}

func (s *TwilioNotificationSender) SendNotification(ctx *gin.Context, content, phoneNumber, countryCode string) error {
	client := twilio.NewClient(s.Config.Verification().TwilioAccountSID(), s.Config.Verification().TwilioAuthToken(), s.HTTPClient)
	from, ok := s.SenderIDs[countryCode]
	if !ok {
		from = s.Config.Verification().TwilioFromNumber()
	}

	msg, err := client.Messages.SendMessage(from, phoneNumber, content, nil)
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
