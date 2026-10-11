package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// WhatsApp sends a short alert to a phone number.
type WhatsApp interface {
	SendWhatsApp(ctx context.Context, to, title, body string) error
}

// LogWhatsApp stands in for WhatsApp until a Business number is registered. It is the default in
// production too, so it records that an alert was skipped without the number or the text, which
// carry people's phone numbers and guest names.
type LogWhatsApp struct {
	Logger *slog.Logger
}

// SendWhatsApp logs that the alert wasn't sent, with the number masked.
func (l LogWhatsApp) SendWhatsApp(_ context.Context, to, _, _ string) error {
	l.Logger.Info("whatsapp alert not sent (log provider)", "to", maskPhone(to))
	return nil
}

// maskPhone keeps a number's last three digits, enough to tell team members apart in a log.
func maskPhone(phone string) string {
	if len(phone) <= 3 {
		return "***"
	}
	return "***" + phone[len(phone)-3:]
}

// graphURL is the Cloud API messages endpoint, on the Graph API version current when this was
// written. Check Meta's changelog when turning WhatsApp on.
const graphURL = "https://graph.facebook.com/v23.0/%s/messages"

// CloudWhatsApp sends alerts through the WhatsApp Cloud API. Messages a business starts must use
// an approved template; Template names one whose body takes two parameters, the alert title and
// its detail line.
type CloudWhatsApp struct {
	Token         string
	PhoneNumberID string
	Template      string
	Language      string // default "en"

	Client *http.Client // default: 10 second timeout
	URL    string       // default: the messages endpoint for PhoneNumberID
}

type waText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type waMessage struct {
	MessagingProduct string `json:"messaging_product"`
	To               string `json:"to"`
	Type             string `json:"type"`
	Template         struct {
		Name     string `json:"name"`
		Language struct {
			Code string `json:"code"`
		} `json:"language"`
		Components []struct {
			Type       string   `json:"type"`
			Parameters []waText `json:"parameters"`
		} `json:"components"`
	} `json:"template"`
}

// SendWhatsApp sends the alert template to an E.164 number.
func (c CloudWhatsApp) SendWhatsApp(ctx context.Context, to, title, body string) error {
	endpoint, client, lang := c.URL, c.Client, c.Language
	if endpoint == "" {
		endpoint = fmt.Sprintf(graphURL, c.PhoneNumberID)
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if lang == "" {
		lang = "en"
	}
	msg := waMessage{MessagingProduct: "whatsapp", To: strings.TrimPrefix(to, "+"), Type: "template"}
	msg.Template.Name = c.Template
	msg.Template.Language.Code = lang
	msg.Template.Components = append(msg.Template.Components, struct {
		Type       string   `json:"type"`
		Parameters []waText `json:"parameters"`
	}{Type: "body", Parameters: []waText{{"text", title}, {"text", body}}})

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("whatsapp: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("whatsapp: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("whatsapp: http status %d", res.StatusCode)
	}
	return nil
}
