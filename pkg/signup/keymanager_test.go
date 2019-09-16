package signup_test

import (
	"fmt"
	//"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
		// test key
		keyJSON = `{"keys":[
			{"kid":"key0","kty":"RSA","e":"AQAB","n":"nzyis1ZjfNB0bBgKFMSvvkTtwlvBsaJq7S5wA-kzeVOVpVWwkWdVha4s38XM_pa_yr47av7-z3VTmvDRyAHcaT92whREFpLv9cj5lTeJSibyr_Mrm_YtjCZVWgaOYIhwrXwKLqPr_11inWsAkfIytvHWTxZYEcXLgAXFuUuaS3uF9gEiNQwzGTU1v0FqkqTBr4B8nW3HCN47XUu0t8Y0e-lf4s4OxQawWD79J9_5d3Ry0vbV3Am1FtGJiJvOwRsIfVChDpYStTcHTCMqtvWbV6L11BWkpzGXSW4Hv43qa-GSYOD2QU68Mb59oSk2OB-BtOLpJofmbGEGgvmwyCI9Mw"},
			{"kid":"key1","kty":"RSA","e":"AQAB","n":"4niTFsMZ_gLOcg9OuwMK4LpBzpdS8ulIGmx5B4rNqWVHAWMpg4kEmZTQffVmKmiw3NUDSaSWLcLJp22ekN2sj1E7tEu1pJksYsXNDa3WLaE1uqVeso-HVv2rIbucd5xMaryvf490g2I-PSrZdSvN73VqJM525s7pPanxe1skqh8"}
		]}`
		keyKID0 = "key0"
		keyKID1 = "key1"
		// test JWT, signed with above keys
		jwt0 = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImtleTAifQ.eyJqdGkiOiIwMjgyYjI5Yy01MTczLTQyZDgtODE0NS1iNDVmYTFlMzUzOGIiLCJleHAiOjAsIm5iZiI6MCwiaWF0IjoxNTE3MDE1OTUyLCJpc3MiOiJ0ZXN0IiwiYXVkIjoiY29kZXJlYWR5LXJlZ2lzdHJhdGlvbi1zZXJ2aWNlIiwic3ViIjoiMjM5ODQzOTgtODU1YS00MmQ2LWE3ZmUtOTM2YmI0ZTkyYTBjIiwidHlwIjoiQmVhcmVyIiwic2Vzc2lvbl9zdGF0ZSI6ImVhZGMwNjZjLTEyMzQtNGE1Ni05ZjM1LWNlNzA3YjU3YTRlOSIsImFjciI6IjAiLCJhbGxvd2VkLW9yaWdpbnMiOlsiKiJdLCJhcHByb3ZlZCI6dHJ1ZSwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5hbWUiOiJUZXN0MCBVc2VyMCIsImNvbXBhbnkiOiJUZXN0IENvbXBhbnkgMCIsInByZWZlcnJlZF91c2VybmFtZSI6InRlc3R1c2VyMCIsImdpdmVuX25hbWUiOiJUZXN0MCIsImZhbWlseV9uYW1lIjoiVXNlcjAiLCJlbWFpbCI6InRlc3R1c2VyMEB0ZXN0LnQifQ.a2JbOJXEB7IAK4JUEcW886VNJoQeHJ_yQiN5dgoPECpKw7PdQAajVBEnSRj6irDsNCjBznDnthSg_rfRv10_cN5K--FoB3eynylMm2Zwdj1JiVpiYd-lK0OfdLUfXlWalq-GLAoiI0UzDBjQE_Izzr58KaeYzjP7Gb9et99zVLDpnmLCoa9vCrXL7G-Dir4l2CqVtorp653Y19EY-A4TeJEitrqg4Uc8SXGskgpXnDZogX0IzFnwvDXawxeUUUoVaSugaxeyAh-ZyFMOFMtHXOM5M-7_3_kMMeVlKrCrbMRN6as5-jQ2vyrZND6fnVuZhrGgoIZNYfN5qaJHgpMikQ"
		jwt1 = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImtleTEifQ.eyJqdGkiOiIwMjgyYjI5Yy01MTczLTQyZDgtODE0NS1iNDVmYTFlMzUzOGMiLCJleHAiOjAsIm5iZiI6MCwiaWF0IjoxNTE3MDE1OTUyLCJpc3MiOiJ0ZXN0IiwiYXVkIjoiY29kZXJlYWR5LXJlZ2lzdHJhdGlvbi1zZXJ2aWNlIiwic3ViIjoiMjM5ODQzOTgtODU1YS00MmQ2LWE3ZmUtOTM2YmI0ZTkyYTBkIiwidHlwIjoiQmVhcmVyIiwic2Vzc2lvbl9zdGF0ZSI6ImVhZGMwNjZjLTEyMzQtNGE1Ni05ZjM1LWNlNzA3YjU3YTRlMCIsImFjciI6IjAiLCJhbGxvd2VkLW9yaWdpbnMiOlsiKiJdLCJhcHByb3ZlZCI6dHJ1ZSwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5hbWUiOiJUZXN0MSBVc2VyMSIsImNvbXBhbnkiOiJUZXN0IENvbXBhbnkgMSIsInByZWZlcnJlZF91c2VybmFtZSI6InRlc3R1c2VyMSIsImdpdmVuX25hbWUiOiJUZXN0MSIsImZhbWlseV9uYW1lIjoiVXNlcjEiLCJlbWFpbCI6InRlc3R1c2VyMUB0ZXN0LnQifQ.XoLc1_ESvotJwwfK-zsyu4wySeFalGuHWB1cHRVBPKmztOPgQUGOb4zplhyKAuPPf0x3WGV50ESNW9t-YZoD-DJceMJY_AOzt0LBAoGoeovsVHbrHNGgMDEJsgjQd_beMQsqVGeJrReN9hnqZ8iPz1itMzzTUskG1TQylcr_ez4"
)

func TestKeyFetching(t *testing.T) {
	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", false)
	assert.False(t, configRegistry.IsTestingMode(), "testing mode not set correctly to false")

	t.Run("parse keys, valid response", func(t *testing.T) {
		// setup http service serving the test keys
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, keyJSON)
		}))
		defer ts.Close()

		// check if service runs
		_, err := http.Get(ts.URL)
		require.NoError(t, err)

		// Set the config for testing mode, the handler may use this.
		configRegistry.GetViperInstance().Set("auth_client.public_keys_url", ts.URL)
		assert.Equal(t, configRegistry.GetAuthClientPublicKeysURL(), ts.URL, "key url not set correctly for testing")

		// Create KeyManager instance.
		keyManager, err := signup.NewKeyManager(logger, configRegistry)
		require.NoError(t, err)

		// just check if the keys are parsed correctly
		_, err = keyManager.Key(keyKID0)
		require.NoError(t, err)
		_, err = keyManager.Key(keyKID1)
		require.NoError(t, err)
	})

	t.Run("parse keys, invalid response", func(t *testing.T) {
		// setup http service serving the test keys
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{some: "invalid", "json"}`)
		}))
		defer ts.Close()

		// check if service runs
		_, err := http.Get(ts.URL)
		require.NoError(t, err)

		// Set the config for testing mode, the handler may use this.
		configRegistry.GetViperInstance().Set("auth_client.public_keys_url", ts.URL)
		assert.Equal(t, configRegistry.GetAuthClientPublicKeysURL(), ts.URL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err = signup.NewKeyManager(logger, configRegistry)
		// this needs to fail with an error
		require.Error(t, err)
	})
}
func TestKeyManager(t *testing.T) {
	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	configRegistry.GetViperInstance().Set("testkeys", keyJSON)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	// Create KeyManager instance.
	keyManager, err := signup.NewKeyManager(logger, configRegistry)
	require.NoError(t, err)

	t.Run("parse keys", func(t *testing.T) {
		// just check if the keys are parsed correctly
		_, err := keyManager.Key(keyKID0)
		require.NoError(t, err)
		_, err = keyManager.Key(keyKID1)
		require.NoError(t, err)
	})

	t.Run("validate with valid keys", func(t *testing.T) {
		// check if the keys can be used to verify a JWT
		var statictests = []struct {
			name string
			jwt  string
			kid  string
		}{
			{"JWT0", jwt0, keyKID0},
			{"JWT1", jwt1, keyKID1},
		}
		for _, tt := range statictests {
			t.Run(tt.name, func(t *testing.T) {
				_, err = jwt.Parse(tt.jwt, func(token *jwt.Token) (interface{}, error) {
					kid := token.Header["kid"]
					require.Equal(t, tt.kid, kid)
					return keyManager.Key(kid.(string))
				})
				require.NoError(t, err)
			})
		}
	})

	t.Run("validate with invalid keys", func(t *testing.T) {
		// check if the verification fails on wrong keys
		var statictests = []struct {
			name     string
			jwt      string
			kidToUse string
		}{
			// keys are swapped here to get invalid keys
			{"JWT0", jwt0, keyKID1},
			{"JWT1", jwt1, keyKID0},
		}
		for _, tt := range statictests {
			t.Run(tt.name, func(t *testing.T) {
				_, err = jwt.Parse(tt.jwt, func(token *jwt.Token) (interface{}, error) {
					return keyManager.Key(tt.kidToUse)
				})
				require.Error(t, err)
			})
		}
	})
}
