package sender

import (
	"fmt"
	"net/http"

	twilioclient "github.com/twilio/twilio-go"
	twiliohttp "github.com/twilio/twilio-go/client"
	openapi "github.com/twilio/twilio-go/rest/lookups/v2"
)

// PhoneLookupResultDetails holds supplementary lookup data stored as a JSON annotation.
type PhoneLookupResultDetails struct {
	RiskScore   int    `json:"risk_score"`
	CarrierName string `json:"carrier_name"`
	LineType    string `json:"line_type"`
}

// PhoneLookupResult holds the parsed response from a Twilio Lookup v2 API call.
type PhoneLookupResult struct {
	PhoneLookupResultDetails
	CarrierRiskCategory string
	NumberBlocked       bool
	CountryCode         string
}

// PhoneLooker checks phone numbers for fraud risk before SMS verification.
type PhoneLooker interface {
	LookupPhone(phoneNumber string) (*PhoneLookupResult, error)
}

// TwilioPhoneLookup implements PhoneLooker using the Twilio Lookup v2 API.
type TwilioPhoneLookup struct {
	client *twilioclient.RestClient
}

// NewTwilioPhoneLookup creates a PhoneLooker backed by the Twilio Lookup v2 API.
func NewTwilioPhoneLookup(accountSID, authToken string, httpClient *http.Client) PhoneLooker {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	baseClient := &twiliohttp.Client{
		Credentials: twiliohttp.NewCredentials(accountSID, authToken),
		HTTPClient:  httpClient,
	}
	client := twilioclient.NewRestClientWithParams(twilioclient.ClientParams{
		Username: accountSID,
		Password: authToken,
		Client:   baseClient,
	})
	return &TwilioPhoneLookup{client: client}
}

// LookupPhone fetches sms_pumping_risk and line_type_intelligence for the given E.164 number.
func (t *TwilioPhoneLookup) LookupPhone(phoneNumber string) (*PhoneLookupResult, error) {
	params := &openapi.FetchPhoneNumberParams{}
	params.SetFields("sms_pumping_risk,line_type_intelligence")

	resp, err := t.client.LookupsV2.FetchPhoneNumber(phoneNumber, params)
	if err != nil {
		return nil, fmt.Errorf("twilio phone lookup: %w", err)
	}

	result := &PhoneLookupResult{}
	if resp.CountryCode != nil {
		result.CountryCode = *resp.CountryCode
	}
	result.CarrierRiskCategory = resp.SmsPumpingRisk.CarrierRiskCategory
	result.NumberBlocked = resp.SmsPumpingRisk.NumberBlocked
	result.RiskScore = resp.SmsPumpingRisk.SmsPumpingRiskScore
	result.CarrierName = resp.LineTypeIntelligence.CarrierName
	result.LineType = resp.LineTypeIntelligence.Type

	return result, nil
}
