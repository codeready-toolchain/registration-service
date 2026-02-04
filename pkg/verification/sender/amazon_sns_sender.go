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

type AmazonSNSSender struct {
	Config AWSSenderConfiguration
}

func NewAmazonSNSSender(cfg AWSSenderConfiguration) NotificationSender {
	return &AmazonSNSSender{
		Config: cfg,
	}
}

func (s *AmazonSNSSender) SendNotification(_ *gin.Context, content, phoneNumber, _ string) error {
	// TODO add support for country-specific sender IDs if we ever decide to use Amazon SNS to send notifications

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
