package sender

import (
	"net/http"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

type NotificationSender interface {
	SendNotification(ctx *gin.Context, content, phoneNumber, countryCode string) error
}

type NotificationSenderOption = func()

func CreateNotificationSender(httpClient *http.Client) NotificationSender {
	cfg := configuration.GetRegistrationServiceConfig()
	if strings.ToLower(cfg.Verification().NotificationSender()) == "aws" {
		return NewAmazonSNSSender(cfg.Verification())
	}

	return NewTwilioSender(cfg, httpClient)
}
