package sender

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

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

// Matches the status Twilio embeds when the error response body cannot be decoded:
// "error decoding the response for an HTTP error code: 503".
var httpStatusInTwilioErrMsg = regexp.MustCompile(`HTTP error code: (\d{3})`)

// LookupErrorType returns a metric label for a phone lookup failure.
// Prefer the HTTP status code from Twilio REST errors (e.g. "500", "503");
// fall back to parsing the status from Twilio's decode-error message, then "unknown".
func LookupErrorType(err error) string {
	if err == nil {
		return "unknown"
	}
	var restErr *twiliohttp.TwilioRestError
	if errors.As(err, &restErr) && restErr.Status != 0 {
		return strconv.Itoa(restErr.Status)
	}
	var restErrV1 *twiliohttp.RestErrorV1
	if errors.As(err, &restErrV1) && restErrV1.HttpStatusCode != 0 {
		return strconv.Itoa(restErrV1.HttpStatusCode)
	}
	if m := httpStatusInTwilioErrMsg.FindStringSubmatch(err.Error()); len(m) == 2 {
		return m[1]
	}
	return "unknown"
}
