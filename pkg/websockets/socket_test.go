package websockets_test

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	testutils "github.com/codeready-toolchain/registration-service/test"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestWebsocketsSuite struct {
	tokengenerator *testutils.TokenManager
	srv *server.RegistrationServer
	testutils.UnitTestSuite
}

func TestRunWebsocketsSuite(t *testing.T) {
	suite.Run(t, &TestWebsocketsSuite{
		nil,
		nil,
		testutils.UnitTestSuite{},
	})
}

func (s *TestWebsocketsSuite) setupConnection() (string, error) {
	// setting up service and routes.
	var err error
	s.srv, err = server.New("")
	if err != nil {
		return "", err
	}
	// create a TokenGenerator and a key
	s.tokengenerator = testutils.NewTokenManager()
	kid0 := uuid.NewV4().String()
	_, err = s.tokengenerator.AddPrivateKey(kid0)
	require.NoError(s.T(), err)
	// start key service
	keysEndpointURL := s.tokengenerator.NewKeyServer().URL
	// set the key service url in the config
	os.Setenv(configuration.EnvPrefix+"_"+"AUTH_CLIENT_PUBLIC_KEYS_URL", keysEndpointURL)
	assert.Equal(s.T(), keysEndpointURL, s.srv.Config().GetAuthClientPublicKeysURL(), "key url not set correctly")
	os.Setenv(configuration.EnvPrefix+"_"+"TESTINGMODE", "true")
	assert.True(s.T(), s.srv.Config().IsTestingMode(), "testing mode not set correctly")
	// setup routes
	err = s.srv.SetupRoutes()
	if err != nil {
		return "", err
	}
	// run the server. Note that this needs to be by "manually"
	// launching the actual HTTPServer as the websockets connection
	// will take over the control of the network socket. This causes
	// it to be non-compatible with the usual httptest procedures.
	go func() {
		s.srv.HTTPServer().ListenAndServe()
	}()
	// let the system settle and the websockets library take over
	// and setup the connection.
	time.Sleep(50)
	return kid0, nil
}

func (s *TestWebsocketsSuite) connect(token string, useURLParam bool) (*websocket.Conn, error) {
	// connect to the websocket service.
	address := strings.Replace(s.srv.Config().GetHTTPAddress(), "0.0.0.0", "127.0.0.1", 1)
	url := "ws://"+address+"/ws"
	requestHeader := http.Header{}
	if token != "" {
		if useURLParam {
			// add token to url
			url = url + "?token=" + token
		} else {
			// setup request header with bearer token
			requestHeader["Authorization"] = []string {"Bearer " + token}
		}	
	}
	ws, _, err := websocket.DefaultDialer.Dial(url, requestHeader)
	if err != nil {
		return nil, err
	}
	// return conn, and keyId
	return ws, nil
}

func (s *TestWebsocketsSuite) TestWebsocketsHub() {
	// create service
	kid, err := s.setupConnection()
	require.NoError(s.T(), err)
	require.NotEqual(s.T(), "", kid)
	defer func() {
		s.srv.HTTPServer().Close()
	}()

	// note that the middleware and the token acceptance is tested in
	// middleware_test.go. This test only tests if the wesockets 
	// connection also uses the middleware.

	s.Run("unauthorized no token", func() {
		conn, err := s.connect("", false)
		require.Nil(s.T(), conn)
		require.Equal(s.T(), "websocket: bad handshake", err.Error())	
	})

	s.Run("unauthorized invalid token - url", func() {
		conn, err := s.connect(uuid.NewV4().String(), true)
		require.Nil(s.T(), conn)
		require.Equal(s.T(), "websocket: bad handshake", err.Error())	
	})

	s.Run("unauthorized invalid token - header", func() {
		conn, err := s.connect(uuid.NewV4().String(), false)
		require.Nil(s.T(), conn)
		require.Equal(s.T(), "websocket: bad handshake", err.Error())	
	})

	s.Run("close connection from client", func() {
		// create a valid test token for echotest
		identity := testutils.Identity{
			ID:       uuid.NewV4(),
			Username: uuid.NewV4().String(),
		}
		tokenValidEchotest, err := s.tokengenerator.GenerateSignedToken(identity, kid, testutils.WithEmailClaim(uuid.NewV4().String()+"@email.tld"))
		require.NoError(s.T(), err)
		// connect
		conn, err := s.connect(tokenValidEchotest, false)
		require.NotNil(s.T(), conn)
		require.Nil(s.T(), err)
		// let the async message processing settle
		time.Sleep(500)
		// now the client map should have the client
		require.Len(s.T(), s.srv.WebsocketsHandler().Hub().Clients(), 1)
		for _, sub := range s.srv.WebsocketsHandler().Hub().Clients() { 
			require.Equal(s.T(), identity.ID.String(), sub)
		}
		// close connection
		err = conn.Close()
		require.NoError(s.T(), err)
		// let the async message processing settle
		time.Sleep(500)
		// now the client map should be empty
		require.Len(s.T(), s.srv.WebsocketsHandler().Hub().Clients(), 0)
	})

/*
	s.Run("authorized echo request - header", func() {
		// create a valid test token for echotest
		identity := testutils.Identity{
			ID:       uuid.NewV4(),
			Username: uuid.NewV4().String(),
		}
		tokenValidEchotest, err := s.tokengenerator.GenerateSignedToken(identity, kid, testutils.WithEmailClaim(uuid.NewV4().String()+"@email.tld"))
		require.NoError(s.T(), err)
		// connect
		conn, err := s.connect(tokenValidEchotest, false)
		require.NotNil(s.T(), conn)
		require.Nil(s.T(), err)
		// close connection when done
		defer func() {
			conn.Close()
		}()
		// send test message to echotest
		testMessage := uuid.NewV4().String()
		err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
		require.NoError(s.T(), err)
		_, p, err := conn.ReadMessage()
		require.NoError(s.T(), err)
		// check if response has the correct subject identified (taken from the token) and the message.
		expected := `{ "sub": "` + identity.ID.String() + `", "body": "` + testMessage + `" }`
		require.Equal(s.T(), []byte(expected), p)		
	})
*/
/*
	s.Run("authorized echo request - url", func() {
		// create a valid test token for echotest
		identity := testutils.Identity{
			ID:       uuid.NewV4(),
			Username: uuid.NewV4().String(),
		}
		tokenValidEchotest, err := s.tokengenerator.GenerateSignedToken(identity, kid, testutils.WithEmailClaim(uuid.NewV4().String()+"@email.tld"))
		require.NoError(s.T(), err)
		// connect
		conn, err := s.connect(tokenValidEchotest, true)
		require.NotNil(s.T(), conn)
		require.Nil(s.T(), err)
		// close connection when done
		defer func() {
			conn.Close()
		}()
		// send test message to echotest
		testMessage := uuid.NewV4().String()
		err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
		require.NoError(s.T(), err)
		_, p, err := conn.ReadMessage()
		require.NoError(s.T(), err)
		// check if response has the correct subject identified (taken from the token) and the message.
		expected := `{ "sub": "` + identity.ID.String() + `", "body": "` + testMessage + `" }`
		require.Equal(s.T(), []byte(expected), p)
	})
*/
}
