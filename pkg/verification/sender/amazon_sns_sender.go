package sender

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"

	"github.com/gin-gonic/gin"
)

type AWSSenderConfiguration interface {
	AWSAccessKeyID() string
	AWSSecretAccessKey() string
	AWSRegion() string
	AWSSenderID() string
	AWSSMSType() string
}

type amazonSNSSender struct {
	Config AWSSenderConfiguration
}

func NewAmazonSNSSender(cfg AWSSenderConfiguration) NotificationSender {
	return &amazonSNSSender{
		Config: cfg,
	}
}

func (s *amazonSNSSender) SendNotification(ctx *gin.Context, content, phoneNumber string) error {
	awsAccessKeyID := s.Config.AWSAccessKeyID()
	awsSecretAccessKey := s.Config.AWSSecretAccessKey()

	creds := credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, "")

	sess, err := session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      aws.String(s.Config.AWSRegion())},
	)

	if err != nil {
		return err
	}

	svc := sns.New(sess)

	senderID := &sns.MessageAttributeValue{}
	senderID.SetDataType("String")
	senderID.SetStringValue(s.Config.AWSSenderID())

	smsType := &sns.MessageAttributeValue{}
	smsType.SetDataType("String")
	smsType.SetStringValue(s.Config.AWSSMSType())

	_, err = svc.Publish(&sns.PublishInput{
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

	return nil
}
