package sender

import (
	"net/http"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/labstack/echo/v4"
)

type NotificationSender interface {
	SendNotification(ctx echo.Context, content, phoneNumber, countryCode string) error
}

type NotificationSenderOption = func()

func CreateNotificationSender(httpClient *http.Client) NotificationSender {
	cfg := configuration.GetRegistrationServiceConfig()
	if strings.ToLower(cfg.Verification().NotificationSender()) == "aws" {
		return NewAmazonSNSSender(cfg.Verification())
	}

	return NewTwilioSender(cfg.Verification(), httpClient)
}
