package signup

import (
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func CreateMessage(to, from string, body []string) strings.Reader {
	// Set up rand
	rand.Seed(time.Now().Unix())

	msgData := url.Values{}
	msgData.Set("To", to)
	msgData.Set("From", from)
	msgData.Set("Body", body[rand.Intn(len(body))])

	return *strings.NewReader(msgData.Encode())
}

func Send(msg strings.Reader) *http.Response {
	// Set account keys & information
	accountSid := "ACXXXX"
	authToken := "XXXXXX"
	urlStr := "https://api.twilio.com/2010-04-01/Accounts/" + accountSid + "/Messages.json"

	client := &http.Client{}
	req, _ := http.NewRequest("POST", urlStr, &msg)
	req.SetBasicAuth(accountSid, authToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, _ := client.Do(req)
	return resp
}
