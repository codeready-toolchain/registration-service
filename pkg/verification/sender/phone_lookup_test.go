package sender_test

import (
	"net/http"
	"testing"

	sender2 "github.com/codeready-toolchain/registration-service/pkg/verification/sender"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
)

func TestTwilioPhoneLookup(t *testing.T) {
	const (
		accountSID = "ACaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		authToken  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		phone      = "+447700900000"
	)

	setupLookup := func(t *testing.T, status int, body interface{}) sender2.PhoneLookupService {
		t.Helper()
		httpClient := &http.Client{Transport: &http.Transport{}}
		gock.InterceptClient(httpClient)
		t.Cleanup(gock.Off)

		gock.New("https://lookups.twilio.com").
			Get("/v2/PhoneNumbers/"+phone).
			MatchParam("Fields", "sms_pumping_risk,line_type_intelligence").
			Reply(status).
			JSON(body).
			SetHeader("Content-Type", "application/json")
		return sender2.NewTwilioPhoneLookup(accountSID, authToken, httpClient)
	}

	t.Run("high-risk response parsing", func(t *testing.T) {
		lookup := setupLookup(t, http.StatusOK, map[string]interface{}{
			"country_code": "GB",
			"sms_pumping_risk": map[string]interface{}{
				"carrier_risk_category":  "high",
				"number_blocked":         true,
				"sms_pumping_risk_score": 34,
			},
			"line_type_intelligence": map[string]interface{}{
				"carrier_name": "Test Carrier",
				"type":         "mobile",
			},
		})

		result, err := lookup.LookupPhone(t.Context(), phone)
		require.NoError(t, err)
		assert.Equal(t, "high", result.CarrierRiskCategory)
		assert.True(t, result.NumberBlocked)
		assert.Equal(t, 34, result.RiskScore)
		assert.Equal(t, "Test Carrier", result.CarrierName)
		assert.Equal(t, "mobile", result.LineType)
		assert.Equal(t, "GB", result.CountryCode)
	})

	t.Run("low-risk response parsing", func(t *testing.T) {
		lookup := setupLookup(t, http.StatusOK, map[string]interface{}{
			"sms_pumping_risk": map[string]interface{}{
				"carrier_risk_category":  "low",
				"number_blocked":         false,
				"sms_pumping_risk_score": 2,
			},
			"line_type_intelligence": map[string]interface{}{
				"carrier_name": "O2",
				"type":         "landline",
			},
		})

		result, err := lookup.LookupPhone(t.Context(), phone)
		require.NoError(t, err)
		assert.Equal(t, "low", result.CarrierRiskCategory)
		assert.False(t, result.NumberBlocked)
		assert.Equal(t, 2, result.RiskScore)
		assert.Equal(t, "O2", result.CarrierName)
		assert.Equal(t, "landline", result.LineType)
		assert.Empty(t, result.CountryCode)
	})

	t.Run("API error handling", func(t *testing.T) {
		lookup := setupLookup(t, http.StatusInternalServerError, map[string]interface{}{
			"message": "Internal Server Error",
		})

		result, err := lookup.LookupPhone(t.Context(), phone)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "twilio phone lookup")
	})

	t.Run("response with missing fields", func(t *testing.T) {
		lookup := setupLookup(t, http.StatusOK, map[string]interface{}{
			"phone_number": phone,
		})

		result, err := lookup.LookupPhone(t.Context(), phone)
		require.NoError(t, err)
		assert.Empty(t, result.CarrierRiskCategory)
		assert.False(t, result.NumberBlocked)
		assert.Zero(t, result.RiskScore)
		assert.Empty(t, result.CarrierName)
		assert.Empty(t, result.LineType)
		assert.Empty(t, result.CountryCode)
	})
}
