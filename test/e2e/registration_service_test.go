package e2e

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"fmt"
	"os"
	"net/http/httptest"

	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
)

func TestRegistrationService(t *testing.T) {
	log.Init("registration-service", nil)
	t.Run("verify_healthcheck", func(t *testing.T) {
		resp, err := http.Get("http://localhost:8080/api/v1/health")
		require.Nil(t, err)
		require.NotNil(t, resp)

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		require.Nil(t, err)
		require.NotNil(t, body)

		mp := make(map[string]interface{})
		err = json.Unmarshal([]byte(body), &mp)
		require.Nil(t, err)

		alive := mp["alive"]
		require.True(t, alive.(bool))

		testingMode := mp["testingMode"]
		require.False(t, testingMode.(bool))

		revision := mp["revision"]
		require.NotNil(t, revision)

		buildTime := mp["buildTime"]
		require.NotNil(t, buildTime)

		startTime := mp["startTime"]
		require.NotNil(t, startTime)
	})

	t.Run("verify_authconfig", func(t *testing.T) {
		resp, err := http.Get("http://localhost:8080/api/v1/authconfig")
		require.Nil(t, err)
		require.NotNil(t, resp)

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))
		require.Nil(t, err)
		require.NotNil(t, body)

		mp := make(map[string]interface{})
		err = json.Unmarshal([]byte(body), &mp)
		require.Nil(t, err)

		alive := mp["auth-client-library-url"]
		require.Equal(t, alive.(string), "https://keycloak.service/auth/js/keycloak.js")

		testingMode := mp["auth-client-config"].(string)
		mp1 := make(map[string]interface{})
		err = json.Unmarshal([]byte(testingMode), &mp1)
		require.Nil(t, err)

		realm := mp1["realm"]
		require.Equal(t, realm.(string), "myRealm")

		authServerURL := mp1["auth-server-url"]
		require.Equal(t, authServerURL.(string), "https://auth.service/auth")

		sslRequired := mp1["ssl-required"]
		require.Equal(t, sslRequired.(string), "none")

		resource := mp1["resource"]
		require.Equal(t, resource.(string), "registrationService")

		publicClient := mp1["public-client"]
		require.True(t, publicClient.(bool))

		confidentialPort := mp1["confidential-port"]
		require.Equal(t, int(confidentialPort.(float64)), 0)
	})

	t.Run("verify_signup_error_no_token", func(t *testing.T) {
		requestBody, err := json.Marshal(map[string]string{})
		require.Nil(t, err)

		resp, err := http.Post("http://localhost:8080/api/v1/signup", "application/json", bytes.NewBuffer(requestBody))
		require.Nil(t, err)
		require.NotNil(t, resp)

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		require.Nil(t, err)
		require.NotNil(t, body)

		mp := make(map[string]interface{})
		err = json.Unmarshal([]byte(body), &mp)
		require.Nil(t, err)

		tokenErr := mp["error"].(string)
		require.Equal(t, "no token found", tokenErr)
	})

	t.Run("verify_signup_error_unknown_auth_header", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/signup", nil)
		require.Nil(t, err)
		req.Header.Set("Authorization", "1223123123")

		resp, err := client.Do(req)
		require.Nil(t, err)
		require.NotNil(t, resp)

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		require.Nil(t, err)
		require.NotNil(t, body)

		mp := make(map[string]interface{})
		err = json.Unmarshal([]byte(body), &mp)
		require.Nil(t, err)

		tokenErr := mp["error"].(string)
		require.Equal(t, "found unknown authorization header:1223123123", tokenErr)
	})

	t.Run("verify_signup_error_invalid_token", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/signup", nil)
		require.Nil(t, err)
		req.Header.Set("Authorization", "Bearer 1223123123")

		resp, err := client.Do(req)
		require.Nil(t, err)
		require.NotNil(t, resp)

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		require.Nil(t, err)
		require.NotNil(t, body)

		mp := make(map[string]interface{})
		err = json.Unmarshal([]byte(body), &mp)
		require.Nil(t, err)

		tokenErr := mp["error"].(string)
		require.Equal(t, "token contains an invalid number of segments", tokenErr)
	})

	t.Run("verify_signup_valid_token", func(t *testing.T) {

		tokenManager := testutils.NewTokenManager()
		kid0 := uuid.NewV4().String()
		
		//1. Create Keypair. --- AddPrivateKey() with kid
		privateKey, err := tokenManager.AddPrivateKey(kid0)

		// 2/3. Create Token. GenerateSignedToken(). Sign Token with Private Key. -- use func SignToken()
		//encodedToken, err := tokenManager.SignToken(token, kid0)
		identity := testutils.NewIdentity()
		emailClaim0 := testutils.WithEmailClaim(uuid.NewV4().String() + "@email.tld")
		token, err := tokenManager.GenerateSignedToken(*identity, kid0, emailClaim0)
		require.Nil(t, err)

		// // 4/5. Convert Public Key to JWK JSON Format and return
		serv := tokenManager.NewJWKServer(privateKey, kid0)
		
		keysEndpointURL := serv.URL
		reg, err := configuration.New("")
		srv := server.New(reg)

		// 6. Set  auth_client.public_keys_url  to that address.
		os.Setenv(configuration.EnvPrefix+"_"+"AUTH_CLIENT_PUBLIC_KEYS_URL", keysEndpointURL)
		os.Setenv(configuration.EnvPrefix+"_"+"TESTINGMODE", "true")

		err = srv.SetupRoutes()
		if err != nil {
			panic(err.Error())
		}

		// 7. Send Token in Header to Service.
		req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", nil)
		require.Nil(t, err)
		req.Header.Set("Authorization", "Bearer " + token)


		resp := httptest.NewRecorder()
		srv.Engine().ServeHTTP(resp, req)

		require.Nil(t, err)
		require.NotNil(t, resp)

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(resp.Body)
		
		// Expecting 500 temporarily as the API is not working yet.
		require.Equal(t, 500, resp.Code)
	})
}