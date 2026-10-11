// Package notify sends alerts to the team: Expo push notifications to their phones, and
// WhatsApp messages when they ask for them.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const expoURL = "https://exp.host/--/api/v2/push/send"

// expoBatch is the most messages Expo takes in one request.
const expoBatch = 100

// Push is one notification to one device.
type Push struct {
	Token string
	Title string
	Body  string
	// Data travels with the notification so a tap can open the right screen.
	Data map[string]string
}

// Expo sends push notifications through Expo's push service.
type Expo struct {
	// AccessToken is only needed when the Expo project requires authorized pushes.
	AccessToken string

	Client *http.Client // default: 10 second timeout
	URL    string       // default: Expo's send endpoint
}

type expoMessage struct {
	To    string            `json:"to"`
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
	Sound string            `json:"sound"`
}

type expoReply struct {
	Data []struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Details struct {
			Error string `json:"error"`
		} `json:"details"`
	} `json:"data"`
}

// Send delivers the notifications and returns the tokens Expo says no longer reach a device,
// so the caller can forget them. Other per-message errors are left for Expo's own retries.
func (e Expo) Send(ctx context.Context, pushes []Push) (dead []string, err error) {
	for start := 0; start < len(pushes); start += expoBatch {
		batch := pushes[start:min(start+expoBatch, len(pushes))]
		gone, err := e.send(ctx, batch)
		if err != nil {
			return dead, err
		}
		dead = append(dead, gone...)
	}
	return dead, nil
}

func (e Expo) send(ctx context.Context, batch []Push) ([]string, error) {
	endpoint, client := e.URL, e.Client
	if endpoint == "" {
		endpoint = expoURL
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	msgs := make([]expoMessage, len(batch))
	for i, p := range batch {
		msgs[i] = expoMessage{To: p.Token, Title: p.Title, Body: p.Body, Data: p.Data, Sound: "default"}
	}
	payload, err := json.Marshal(msgs)
	if err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AccessToken)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("expo push: http status %d", res.StatusCode)
	}
	var reply expoReply
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		return nil, fmt.Errorf("expo push: read reply: %w", err)
	}
	var dead []string
	for i, r := range reply.Data {
		if i < len(batch) && r.Status == "error" && r.Details.Error == "DeviceNotRegistered" {
			dead = append(dead, batch[i].Token)
		}
	}
	return dead, nil
}
