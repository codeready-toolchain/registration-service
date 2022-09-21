package sender

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/gin-gonic/gin"
)

type amazonSNSSender struct {
	Config configuration.RegistrationServiceConfig
}

func NewAmazonSNSSender(cfg configuration.RegistrationServiceConfig) NotificationSender {
	return &amazonSNSSender{
		Config: cfg,
	}
}

func (s *amazonSNSSender) SendNotification(ctx *gin.Context, content, phoneNumber string) error {
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(s.Config.Verification().AWSAccessKeyId(), s.Config.Verification().AWSSecretAccessKey(), ""),
		Region:      aws.String(s.Config.Verification().AWSRegion())},
	)

	if err != nil {
		return err
	}

	svc := sns.New(sess)

	senderId := &sns.MessageAttributeValue{}
	senderId.SetDataType("String")
	senderId.SetStringValue(s.Config.Verification().AWSSenderID())

	smsType := &sns.MessageAttributeValue{}
	smsType.SetDataType("String")
	smsType.SetStringValue(s.Config.Verification().AWSSMSType())

	result, err := svc.Publish(&sns.PublishInput{
		Message:     &content,
		PhoneNumber: &phoneNumber,
		MessageAttributes: map[string]*sns.MessageAttributeValue{
			"AWS.SNS.SMS.SenderID": senderId,
			"AWS.SNS.SMS.SMSType":  smsType,
		},
	})

	if err != nil {
		return err
	}

	log.Info(ctx, fmt.Sprintf("Notification Message Sent.  Message ID: [%s] Phone Number: [%s]",
		result.MessageId, phoneNumber))

	return nil
}
