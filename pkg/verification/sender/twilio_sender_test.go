package sender_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	sender2 "github.com/codeready-toolchain/registration-service/pkg/verification/sender"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
)

type MockTwilioConfig struct {
	AccountSID    string
	AuthToken     string
	FromNumber    string
	SenderConfigs []toolchainv1alpha1.TwilioSenderConfig
}

func (c *MockTwilioConfig) TwilioAccountSID() string {
	return c.AccountSID
}

func (c *MockTwilioConfig) TwilioAuthToken() string {
	return c.AuthToken
}

func (c *MockTwilioConfig) TwilioFromNumber() string {
	return c.FromNumber
}

func (c *MockTwilioConfig) TwilioSenderConfigs() []toolchainv1alpha1.TwilioSenderConfig {
	return c.SenderConfigs
}

func TestTwilioSenderID(t *testing.T) {
	cfg := &MockTwilioConfig{
		AccountSID: "TWILIO_SID_VALUE",
		AuthToken:  "AUTH_TOKEN_VALUE",
		FromNumber: "+13334445555",
		SenderConfigs: []toolchainv1alpha1.TwilioSenderConfig{
			{
				SenderID:     "RED HAT",
				CountryCodes: []string{"44"},
			},
		},
	}

	setupGockAndSendRequest := func(executeSend func(sender sender2.NotificationSender) error) string {
		httpClient := &http.Client{Transport: &http.Transport{}}
		gock.InterceptClient(httpClient)

		defer gock.Off()

		gock.New("https://api.twilio.com").
			Reply(http.StatusNoContent).
			BodyString("")

		var reqBody io.ReadCloser
		obs := func(request *http.Request, _ gock.Mock) {
			reqBody = request.Body
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				require.NoError(t, err)
			}(request.Body)
		}

		gock.Observe(obs)

		sender := sender2.NewTwilioSender(cfg, httpClient)

		err := executeSend(sender)
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(reqBody)
		require.NoError(t, err)
		return buf.String()
	}

	t.Run("test country code in config", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		reqValue := setupGockAndSendRequest(func(sender sender2.NotificationSender) error {
			return sender.SendNotification(ctx, "Test Message", "+440000000000", "44")
		})

		v, err := url.ParseQuery(reqValue)
		require.NoError(t, err)

		require.Equal(t, "Test Message", v.Get("Body"))
		require.Equal(t, "RED HAT", v.Get("From"))
		require.Equal(t, "+440000000000", v.Get("To"))
	})

	t.Run("test country code not in config", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		reqValue := setupGockAndSendRequest(func(sender sender2.NotificationSender) error {
			return sender.SendNotification(ctx, "Test Message", "+611234567890", "61")
		})

		v, err := url.ParseQuery(reqValue)
		require.NoError(t, err)

		require.Equal(t, "Test Message", v.Get("Body"))
		require.Equal(t, "+13334445555", v.Get("From"))
		require.Equal(t, "+611234567890", v.Get("To"))
	})
}
