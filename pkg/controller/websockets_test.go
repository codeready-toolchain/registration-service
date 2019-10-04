package controller_test

import (
	"log"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/websockets"
	testutils "github.com/codeready-toolchain/registration-service/test"

	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestWebsocketsHandlerSuite struct {
	testutils.UnitTestSuite
}

func TestRunWebsocketsHandlerSuite(t *testing.T) {
	suite.Run(t, &TestWebsocketsHandlerSuite{testutils.UnitTestSuite{}})
}

func (s *TestWebsocketsHandlerSuite) TestWebsocketsHandler() {

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// create websockets handler
	websocketsHdlr := controller.NewWebsocketsHandler(logger, s.Config)
	require.NotNil(s.T(), websocketsHdlr.Hub())
	require.NotNil(s.T(), websocketsHdlr.Outbound())

	s.Run("channels", func() {
		message := &websockets.Message{
			Sub:  uuid.NewV4().String(),
			Body: []byte(uuid.NewV4().String()),
		}
		websocketsHdlr.Hub().Inbound <- message
		// TODO this generates an async error "error client not found for sub xyz", this needs to be checked
		// this will not catch the message as the Hub will be first to pick up: incomingMessage := <- websocketsHdlr.Hub().Outbound
	})
}
