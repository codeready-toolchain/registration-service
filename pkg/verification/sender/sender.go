package sender

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

type NotificationSender interface {
	SendNotification(ctx *gin.Context, content, phoneNumber string) error
}

type NotificationSenderOption = func()

func CreateNotificationSender(httpClient *http.Client) NotificationSender {
	cfg := configuration.GetRegistrationServiceConfig()
	if cfg.Verification().NotificationSender() == "aws" {
		return NewAmazonSNSSender(cfg)
	}

	return NewTwilioSender(cfg, httpClient)
}
