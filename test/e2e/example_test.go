package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
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

	t.Run("verify_signup_error_token", func(t *testing.T) {
		requestBody, err := json.Marshal(map[string]string{
			"Authorization": "12312312312313",
		})
		require.Nil(t, err)

		resp, err := http.Post("http://localhost:8080/api/v1/signup", "application/json", bytes.NewBuffer(requestBody))
		require.Nil(t, err)
		require.NotNil(t, resp)

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		require.Nil(t, err)
		require.NotNil(t, body)
		fmt.Println(string(body))

		mp := make(map[string]interface{})
		err = json.Unmarshal([]byte(body), &mp)
		require.Nil(t, err)

		tokenErr := mp["error"].(string)
		require.Equal(t, "no token found", tokenErr)
	})
}
