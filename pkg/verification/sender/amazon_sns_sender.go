package sender

import (
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
	awsAccessKeyID := s.Config.Verification().AWSAccessKeyID()
	awsSecretAccessKey := s.Config.Verification().AWSSecretAccessKey()

	creds := credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, "")

	sess, err := session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      aws.String(s.Config.Verification().AWSRegion())},
	)

	if err != nil {
		return err
	}

	svc := sns.New(sess)

	senderID := &sns.MessageAttributeValue{}
	senderID.SetDataType("String")
	senderID.SetStringValue(s.Config.Verification().AWSSenderID())

	smsType := &sns.MessageAttributeValue{}
	smsType.SetDataType("String")
	smsType.SetStringValue(s.Config.Verification().AWSSMSType())

	out, err := svc.Publish(&sns.PublishInput{
		Message:     &content,
		PhoneNumber: &phoneNumber,
		MessageAttributes: map[string]*sns.MessageAttributeValue{
			"AWS.SNS.SMS.SenderID": senderID,
			"AWS.SNS.SMS.SMSType":  smsType,
		},
	})

	if err != nil {
		return err
	}

	log.Info(ctx, out.String())

	return nil
}
