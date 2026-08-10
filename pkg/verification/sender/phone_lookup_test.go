package sender_test

import (
	"net/http"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/verification/sender"
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

	setupLookup := func(t *testing.T, status int, body interface{}) sender.PhoneLooker {
		t.Helper()
		t.Cleanup(gock.Off)

		gock.New("https://lookups.twilio.com").
			Get("/v2/PhoneNumbers/"+phone).
			MatchParam("Fields", "sms_pumping_risk,line_type_intelligence").
			Reply(status).
			JSON(body).
			SetHeader("Content-Type", "application/json")
		return sender.NewTwilioPhoneLookup(accountSID, authToken, nil)
	}

	t.Run("high-risk response parsing", func(t *testing.T) {
		// given
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

		// when
		result, err := lookup.LookupPhone(phone)

		// then
		require.NoError(t, err)
		assert.Equal(t, "high", result.CarrierRiskCategory)
		assert.True(t, result.NumberBlocked)
		assert.Equal(t, 34, result.RiskScore)
		assert.Equal(t, "Test Carrier", result.CarrierName)
		assert.Equal(t, "mobile", result.LineType)
		assert.Equal(t, "GB", result.CountryCode)
	})

	t.Run("low-risk response parsing", func(t *testing.T) {
		// given
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

		// when
		result, err := lookup.LookupPhone(phone)

		// then
		require.NoError(t, err)
		assert.Equal(t, "low", result.CarrierRiskCategory)
		assert.False(t, result.NumberBlocked)
		assert.Equal(t, 2, result.RiskScore)
		assert.Equal(t, "O2", result.CarrierName)
		assert.Equal(t, "landline", result.LineType)
		assert.Empty(t, result.CountryCode)
	})

	t.Run("API error handling", func(t *testing.T) {
		// given — Twilio REST error body includes status so the SDK returns TwilioRestError
		lookup := setupLookup(t, http.StatusInternalServerError, map[string]interface{}{
			"status":    500,
			"code":      20500,
			"message":   "Internal Server Error",
			"more_info": "",
		})

		// when
		result, err := lookup.LookupPhone(phone)

		// then
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "twilio phone lookup")
		assert.Equal(t, "500", sender.LookupErrorType(err))
	})

	t.Run("API error type for service unavailable", func(t *testing.T) {
		// given
		lookup := setupLookup(t, http.StatusServiceUnavailable, map[string]interface{}{
			"status":    503,
			"code":      20503,
			"message":   "Service Unavailable",
			"more_info": "",
		})

		// when
		_, err := lookup.LookupPhone(phone)

		// then
		require.Error(t, err)
		assert.Equal(t, "503", sender.LookupErrorType(err))
	})

	t.Run("API error type from undecodable body", func(t *testing.T) {
		// given — empty/invalid body: Twilio embeds status in the decode-error message
		t.Cleanup(gock.Off)
		gock.New("https://lookups.twilio.com").
			Get("/v2/PhoneNumbers/"+phone).
			MatchParam("Fields", "sms_pumping_risk,line_type_intelligence").
			Reply(http.StatusBadGateway).
			BodyString("not-json")
		lookup := sender.NewTwilioPhoneLookup(accountSID, authToken, nil)

		// when
		_, err := lookup.LookupPhone(phone)

		// then
		require.Error(t, err)
		assert.Equal(t, "502", sender.LookupErrorType(err))
	})

	t.Run("LookupErrorType unknown for non-Twilio errors", func(t *testing.T) {
		assert.Equal(t, "unknown", sender.LookupErrorType(assert.AnError))
	})

	t.Run("response with missing fields", func(t *testing.T) {
		// given
		lookup := setupLookup(t, http.StatusOK, map[string]interface{}{
			"phone_number": phone,
		})

		// when
		result, err := lookup.LookupPhone(phone)

		// then
		require.NoError(t, err)
		assert.Empty(t, result.CarrierRiskCategory)
		assert.False(t, result.NumberBlocked)
		assert.Zero(t, result.RiskScore)
		assert.Empty(t, result.CarrierName)
		assert.Empty(t, result.LineType)
		assert.Empty(t, result.CountryCode)
	})

	t.Run("response with null nested objects", func(t *testing.T) {
		// given
		lookup := setupLookup(t, http.StatusOK, map[string]interface{}{
			"phone_number":           phone,
			"sms_pumping_risk":       nil,
			"line_type_intelligence": nil,
		})

		// when
		result, err := lookup.LookupPhone(phone)

		// then
		require.NoError(t, err)
		assert.Empty(t, result.CarrierRiskCategory)
		assert.False(t, result.NumberBlocked)
		assert.Zero(t, result.RiskScore)
		assert.Empty(t, result.CarrierName)
		assert.Empty(t, result.LineType)
	})

	t.Run("response with empty nested objects", func(t *testing.T) {
		// given
		lookup := setupLookup(t, http.StatusOK, map[string]interface{}{
			"phone_number":           phone,
			"sms_pumping_risk":       map[string]interface{}{},
			"line_type_intelligence": map[string]interface{}{},
		})

		// when
		result, err := lookup.LookupPhone(phone)

		// then
		require.NoError(t, err)
		assert.Empty(t, result.CarrierRiskCategory)
		assert.False(t, result.NumberBlocked)
		assert.Zero(t, result.RiskScore)
		assert.Empty(t, result.CarrierName)
		assert.Empty(t, result.LineType)
	})
}
