package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"staykey.direct/api/internal/domain"
)

const ozoneURL = "https://api.ozonesender.com/v1/send/"

// Ozone sends messages through oZone Sender, a Sri Lankan SMS gateway.
type Ozone struct {
	UserID   string
	APIKey   string
	SenderID string // case sensitive; "oZone Demo" for testing

	Client *http.Client // default: 10 second timeout
	URL    string       // default: the oZone send endpoint
}

// Send texts body to an E.164 number. A number oZone can't deliver to comes back as a
// *domain.ValidationError on the phone field.
func (o Ozone) Send(ctx context.Context, to, body string) error {
	endpoint, client := o.URL, o.Client
	if endpoint == "" {
		endpoint = ozoneURL
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	// The credentials go in the body rather than the query string, so proxies and access logs
	// never record them.
	form := url.Values{
		"user_id":              {o.UserID},
		"api_key":              {o.APIKey},
		"sender_id":            {o.SenderID},
		"recipient_contact_no": {strings.TrimPrefix(to, "+")},
		"message":              {body},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("ozone sms: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ozone sms: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("ozone sms: http status %d", res.StatusCode)
	}

	// The docs show status_code as a number; json.Number also accepts it quoted.
	var reply struct {
		StatusCode json.Number `json:"status_code"`
	}
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		return fmt.Errorf("ozone sms: read reply: %w", err)
	}
	switch reply.StatusCode {
	case "204":
		return nil
	case "203", "205": // invalid number, unsupported operator, or wrong length
		return &domain.ValidationError{Field: "phone", Message: "this number can't receive text messages, check it and try again"}
	default:
		return fmt.Errorf("ozone sms: status %s", reply.StatusCode)
	}
}
