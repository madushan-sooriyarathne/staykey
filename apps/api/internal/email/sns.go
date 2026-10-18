package email

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1" // SNS signature version 1 is SHA1withRSA.
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SNSMessage is a message SNS posts to an HTTPS subscription.
type SNSMessage struct {
	Type             string
	MessageID        string `json:"MessageId"`
	Token            string
	TopicArn         string
	Subject          string
	Message          string
	SubscribeURL     string
	Timestamp        string
	SignatureVersion string
	Signature        string
	SigningCertURL   string
}

// SNS message types.
const (
	SNSNotification         = "Notification"
	SNSSubscriptionConfirm  = "SubscriptionConfirmation"
	SNSUnsubscribeConfirmed = "UnsubscribeConfirmation"
)

// ErrSNSSignature means a message isn't signed by SNS.
var ErrSNSSignature = errors.New("sns signature check failed")

// SNS checks messages from Amazon SNS in one region. Signing certificates are only fetched from
// https://sns.<region>.amazonaws.com and are cached by URL. A valid signature only proves AWS sent
// the message, and anyone can point their own topic at the endpoint, so messages from any topic
// but TopicARN are refused, subscription requests included.
type SNS struct {
	Region   string
	TopicARN string
	// Host overrides the pinned certificate host, for tests.
	Host   string
	Client *http.Client // default: 10 second timeout

	mu    sync.Mutex
	certs map[string]*x509.Certificate
}

// Verify parses body and checks its signature.
func (s *SNS) Verify(ctx context.Context, body []byte) (SNSMessage, error) {
	var m SNSMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return SNSMessage{}, fmt.Errorf("%w: %v", ErrSNSSignature, err)
	}
	var hash crypto.Hash
	switch m.SignatureVersion {
	case "1":
		hash = crypto.SHA1
	case "2":
		hash = crypto.SHA256
	default:
		return SNSMessage{}, fmt.Errorf("%w: signature version %q", ErrSNSSignature, m.SignatureVersion)
	}
	if m.TopicArn != s.TopicARN {
		return SNSMessage{}, fmt.Errorf("%w: message from another topic", ErrSNSSignature)
	}
	if err := s.checkURL(m.SigningCertURL); err != nil {
		return SNSMessage{}, err
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return SNSMessage{}, fmt.Errorf("%w: signature encoding", ErrSNSSignature)
	}
	payload, err := stringToSign(m)
	if err != nil {
		return SNSMessage{}, err
	}
	cert, err := s.cert(ctx, m.SigningCertURL)
	if err != nil {
		return SNSMessage{}, err
	}
	key, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return SNSMessage{}, fmt.Errorf("%w: certificate key is not RSA", ErrSNSSignature)
	}
	var digest []byte
	if hash == crypto.SHA1 {
		d := sha1.Sum([]byte(payload))
		digest = d[:]
	} else {
		d := sha256.Sum256([]byte(payload))
		digest = d[:]
	}
	if err := rsa.VerifyPKCS1v15(key, hash, digest, sig); err != nil {
		return SNSMessage{}, ErrSNSSignature
	}
	return m, nil
}

// ConfirmSubscription opens the message's SubscribeURL, which has to be on the same pinned host.
func (s *SNS) ConfirmSubscription(ctx context.Context, m SNSMessage) error {
	if err := s.checkURL(m.SubscribeURL); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.SubscribeURL, nil)
	if err != nil {
		return fmt.Errorf("confirm subscription: %w", err)
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return fmt.Errorf("confirm subscription: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("confirm subscription: status %d", resp.StatusCode)
	}
	return nil
}

func (s *SNS) host() string {
	if s.Host != "" {
		return s.Host
	}
	return "sns." + s.Region + ".amazonaws.com"
}

// checkURL refuses any URL that isn't https on the pinned host.
func (s *SNS) checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != s.host() || u.User != nil {
		return fmt.Errorf("%w: url is not on %s", ErrSNSSignature, s.host())
	}
	return nil
}

func (s *SNS) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (s *SNS) cert(ctx context.Context, certURL string) (*x509.Certificate, error) {
	s.mu.Lock()
	cert, ok := s.certs[certURL]
	s.mu.Unlock()
	if ok {
		return cert, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, certURL, nil)
	if err != nil {
		return nil, fmt.Errorf("signing certificate: %w", err)
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("signing certificate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("signing certificate: status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("signing certificate: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%w: certificate is not PEM", ErrSNSSignature)
	}
	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSNSSignature, err)
	}
	s.mu.Lock()
	if s.certs == nil {
		s.certs = map[string]*x509.Certificate{}
	}
	s.certs[certURL] = cert
	s.mu.Unlock()
	return cert, nil
}

// stringToSign builds the canonical text SNS signs: field names and values on their own lines,
// in a fixed order that depends on the message type.
func stringToSign(m SNSMessage) (string, error) {
	type field struct{ name, value string }
	var fields []field
	switch m.Type {
	case SNSNotification:
		fields = []field{{"Message", m.Message}, {"MessageId", m.MessageID}}
		if m.Subject != "" {
			fields = append(fields, field{"Subject", m.Subject})
		}
		fields = append(fields, field{"Timestamp", m.Timestamp}, field{"TopicArn", m.TopicArn}, field{"Type", m.Type})
	case SNSSubscriptionConfirm, SNSUnsubscribeConfirmed:
		fields = []field{
			{"Message", m.Message}, {"MessageId", m.MessageID}, {"SubscribeURL", m.SubscribeURL},
			{"Timestamp", m.Timestamp}, {"Token", m.Token}, {"TopicArn", m.TopicArn}, {"Type", m.Type},
		}
	default:
		return "", fmt.Errorf("%w: message type %q", ErrSNSSignature, m.Type)
	}
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(f.name + "\n" + f.value + "\n")
	}
	return b.String(), nil
}

// Suppression is an address SES says not to email again.
type Suppression struct {
	Address string
	Reason  string // bounce or complaint
}

type sesEvent struct {
	NotificationType string `json:"notificationType"`
	EventType        string `json:"eventType"`
	Bounce           struct {
		BounceType        string `json:"bounceType"`
		BouncedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"bouncedRecipients"`
	} `json:"bounce"`
	Complaint struct {
		ComplainedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"complainedRecipients"`
	} `json:"complaint"`
}

// Suppressions reads an SES notification (or a configuration set event) and returns the
// addresses of permanent bounces and complaints. Other events give none.
func Suppressions(message string) ([]Suppression, error) {
	var ev sesEvent
	if err := json.Unmarshal([]byte(message), &ev); err != nil {
		return nil, fmt.Errorf("ses notification: %w", err)
	}
	kind := ev.NotificationType
	if kind == "" {
		kind = ev.EventType
	}
	var out []Suppression
	switch kind {
	case "Bounce":
		if ev.Bounce.BounceType != "Permanent" {
			return nil, nil
		}
		for _, r := range ev.Bounce.BouncedRecipients {
			out = append(out, Suppression{Address: bareAddress(r.EmailAddress), Reason: "bounce"})
		}
	case "Complaint":
		for _, r := range ev.Complaint.ComplainedRecipients {
			out = append(out, Suppression{Address: bareAddress(r.EmailAddress), Reason: "complaint"})
		}
	}
	return out, nil
}

// bareAddress drops a display name SES may include, and lower-cases the address.
func bareAddress(s string) string {
	if i, j := strings.LastIndex(s, "<"), strings.LastIndex(s, ">"); i >= 0 && j > i {
		s = s[i+1 : j]
	}
	return strings.ToLower(strings.TrimSpace(s))
}
