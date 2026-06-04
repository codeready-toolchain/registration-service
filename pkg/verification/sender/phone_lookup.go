package sender

import (
	"context"
	"fmt"
	"net/http"

	twilioclient "github.com/twilio/twilio-go"
	twiliohttp "github.com/twilio/twilio-go/client"
	openapi "github.com/twilio/twilio-go/rest/lookups/v2"
)

// PhoneLookupResult holds the parsed response from a Twilio Lookup v2 API call.
type PhoneLookupResult struct {
	CarrierRiskCategory string
	NumberBlocked       bool
	RiskScore           int
	CarrierName         string
	LineType            string
	CountryCode         string
}

// PhoneLookupService checks phone numbers for fraud risk before SMS verification.
type PhoneLookupService interface {
	LookupPhone(ctx context.Context, phoneNumber string) (*PhoneLookupResult, error)
}

// TwilioPhoneLookup implements PhoneLookupService using the Twilio Lookup v2 API.
type TwilioPhoneLookup struct {
	client *twilioclient.RestClient
}

// NewTwilioPhoneLookup creates a PhoneLookupService backed by the Twilio Lookup v2 API.
func NewTwilioPhoneLookup(accountSID, authToken string, httpClient *http.Client) PhoneLookupService {
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
func (t *TwilioPhoneLookup) LookupPhone(ctx context.Context, phoneNumber string) (*PhoneLookupResult, error) {
	_ = ctx
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
