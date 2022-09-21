package sender

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

type NotificationSender interface {
	SendNotification(ctx *gin.Context, content, phoneNumber string) error
}

func CreateNotificationSender() NotificationSender {
	cfg := configuration.GetRegistrationServiceConfig()
	if cfg.Verification().NotificationSender() == "aws" {
		return NewAmazonSNSSender(cfg)
	} else {
		return NewTwilioSender(cfg)
	}

}
