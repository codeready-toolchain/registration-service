package sender_test

import (
	"bytes"
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	sender2 "github.com/codeready-toolchain/registration-service/pkg/verification/sender"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
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

func TestTwilioSenderIDs(t *testing.T) {

	httpClient := &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(httpClient)

	defer gock.Off()

	gock.New("https://api.twilio.com").
		Reply(http.StatusNoContent).
		BodyString("")

	var reqBody io.ReadCloser
	obs := func(request *http.Request, mock gock.Mock) {
		reqBody = request.Body
		defer request.Body.Close()
	}

	gock.Observe(obs)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	cfg := &MockTwilioConfig{
		AccountSID: "TWILIO_SID_VALUE",
		AuthToken:  "AUTH_TOKEN_VALUE",
		FromNumber: "+13334445555",
		SenderConfigs: []toolchainv1alpha1.TwilioSenderConfig{
			toolchainv1alpha1.TwilioSenderConfig{
				SenderID:     "RED HAT",
				CountryCodes: []string{"44"},
			},
		},
	}

	sender := sender2.NewTwilioSender(cfg, httpClient)

	err := sender.SendNotification(ctx, "Test Message", "+440000000000", "44")
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reqBody)
	require.NoError(t, err)
	reqValue := buf.String()

	v, err := url.ParseQuery(reqValue)
	require.NoError(t, err)

	require.Equal(t, "Test Message", v.Get("Body"))
	require.Equal(t, "RED HAT", v.Get("From"))
	require.Equal(t, "+440000000000", v.Get("To"))

}
