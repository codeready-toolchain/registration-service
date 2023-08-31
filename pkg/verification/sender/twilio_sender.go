package sender

import (
	"fmt"
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-gonic/gin"
	"github.com/kevinburke/twilio-go"
)

type TwilioConfig interface {
	TwilioAccountSID() string
	TwilioAuthToken() string
	TwilioFromNumber() string
	TwilioSenderConfigs() []toolchainv1alpha1.TwilioSenderConfig
}

type TwilioNotificationSender struct {
	Config     TwilioConfig
	HTTPClient *http.Client

	//SenderIDs is a map containing country codes (key) and associated sender id (value)
	SenderIDs map[string]string
}

func NewTwilioSender(cfg TwilioConfig, httpClient *http.Client) NotificationSender {
	sender := &TwilioNotificationSender{
		Config:     cfg,
		HTTPClient: httpClient,
	}

	// Initialize the SenderIDs map
	sender.SenderIDs = make(map[string]string)

	// Populate the SenderIDs map with the configured sender IDs
	for _, senderConfig := range cfg.TwilioSenderConfigs() {
		for _, countryCode := range senderConfig.CountryCodes {
			sender.SenderIDs[countryCode] = senderConfig.SenderID
		}
	}

	return sender
}

func (s *TwilioNotificationSender) SendNotification(ctx *gin.Context, content, phoneNumber, countryCode string) error {
	client := twilio.NewClient(s.Config.TwilioAccountSID(), s.Config.TwilioAuthToken(), s.HTTPClient)
	from, ok := s.SenderIDs[countryCode]
	if !ok {
		from = s.Config.TwilioFromNumber()
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
