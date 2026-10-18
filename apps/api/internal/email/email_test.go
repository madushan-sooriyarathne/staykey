package email

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSESRequest(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MessageId":"ses-1"}`))
	}))
	defer srv.Close()
	s, err := NewSES(SESConfig{
		Region: "ap-south-1", AccessKeyID: "AKIA", SecretAccessKey: "secret", From: "hello@mail.staykey.direct",
		ConfigurationSet: "staykey", Endpoint: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Send(context.Background(), Message{
		To: "guest@example.com", FromName: "Kingfisher Villa via StayKey", ReplyTo: "owner@example.com",
		Subject: "Your stay", Text: "Hello", HTML: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	content := got["Content"].(map[string]any)["Simple"].(map[string]any)
	tests := []struct {
		name string
		got  any
		want string
	}{
		{"message id", id, "ses-1"},
		{"from", got["FromEmailAddress"], `"Kingfisher Villa via StayKey" <hello@mail.staykey.direct>`},
		{"to", got["Destination"].(map[string]any)["ToAddresses"].([]any)[0], "guest@example.com"},
		{"reply-to", got["ReplyToAddresses"].([]any)[0], "owner@example.com"},
		{"configuration set", got["ConfigurationSetName"], "staykey"},
		{"subject", content["Subject"].(map[string]any)["Data"], "Your stay"},
		{"text", content["Body"].(map[string]any)["Text"].(map[string]any)["Data"], "Hello"},
		{"html", content["Body"].(map[string]any)["Html"].(map[string]any)["Data"], "<p>Hello</p>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestSESWithoutReplyToSendsNone(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = w.Write([]byte(`{"MessageId":"ses-2"}`))
	}))
	defer srv.Close()
	s, _ := NewSES(SESConfig{Region: "ap-south-1", AccessKeyID: "a", SecretAccessKey: "b", From: "hello@mail.staykey.direct", Endpoint: srv.URL})
	if _, err := s.Send(context.Background(), Message{To: "a@example.com", FromName: "StayKey", Subject: "s", Text: "t"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["ReplyToAddresses"]; ok {
		t.Errorf("ReplyToAddresses sent: %v", got["ReplyToAddresses"])
	}
}

func TestLogSenderNeverLogsTheMessage(t *testing.T) {
	var buf bytes.Buffer
	l := LogSender{Logger: slog.New(slog.NewTextHandler(&buf, nil))}
	_, _ = l.Send(context.Background(), Message{
		To: "nimal@example.com", Subject: "Hi Nimal", Text: "Your stay at Kingfisher", Link: "https://x/verify/token",
	})
	for _, secret := range []string{"nimal@", "Hi Nimal", "Kingfisher", "token"} {
		t.Run(secret, func(t *testing.T) {
			if strings.Contains(buf.String(), secret) {
				t.Errorf("log %q contains %q", buf.String(), secret)
			}
		})
	}
}

func TestLogSenderShowsLinksInDevelopment(t *testing.T) {
	var buf bytes.Buffer
	l := LogSender{Logger: slog.New(slog.NewTextHandler(&buf, nil)), ShowLinks: true}
	_, _ = l.Send(context.Background(), Message{To: "nimal@example.com", Link: "https://x/verify/token"})
	if !strings.Contains(buf.String(), "https://x/verify/token") {
		t.Errorf("log %q has no link", buf.String())
	}
}

func TestMaskAddress(t *testing.T) {
	if got := MaskAddress("nimal@example.com"); got != "n***@example.com" {
		t.Errorf("MaskAddress = %q", got)
	}
}

func TestBodyEscapesAndLinks(t *testing.T) {
	_, h := Body("Hi <Emma> & co\nhttps://kingfisher.staykey.direct/stay/1?t=a.", "")
	want := `Hi &lt;Emma&gt; &amp; co<br><a href="https://kingfisher.staykey.direct/stay/1?t=a" style="color:#09090b;">https://kingfisher.staykey.direct/stay/1?t=a</a>.`
	if !strings.Contains(h, want) {
		t.Errorf("html = %s, want it to contain %s", h, want)
	}
}

func TestBodyAddsTheFooterToText(t *testing.T) {
	plain, _ := Body("Hello", "Sent by StayKey")
	if plain != "Hello\n\n--\nSent by StayKey" {
		t.Errorf("plain = %q", plain)
	}
}

func TestSNSAcceptsASignedMessage(t *testing.T) {
	f := newSNSFixture(t)
	if _, err := f.sns.Verify(context.Background(), f.sign(t, f.bounce())); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestSNSAcceptsSignatureVersion1(t *testing.T) {
	f := newSNSFixture(t)
	m := f.bounce()
	m.SignatureVersion = "1"
	if _, err := f.sns.Verify(context.Background(), f.sign(t, m)); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestSNSRefusesATamperedMessage(t *testing.T) {
	f := newSNSFixture(t)
	var m SNSMessage
	_ = json.Unmarshal(f.sign(t, f.bounce()), &m)
	m.Message = strings.Replace(m.Message, "guest@example.com", "owner@example.com", 1)
	body, _ := json.Marshal(m)
	if _, err := f.sns.Verify(context.Background(), body); !errors.Is(err, ErrSNSSignature) {
		t.Errorf("Verify error = %v, want ErrSNSSignature", err)
	}
}

func TestSNSRefusesACertificateOnAnotherHost(t *testing.T) {
	f := newSNSFixture(t)
	m := f.bounce()
	m.SigningCertURL = "https://sns.example.com/cert.pem"
	if _, err := f.sns.Verify(context.Background(), f.sign(t, m)); !errors.Is(err, ErrSNSSignature) {
		t.Errorf("Verify error = %v, want ErrSNSSignature", err)
	}
}

func TestSNSRefusesAnotherTopic(t *testing.T) {
	f := newSNSFixture(t)
	m := f.bounce()
	m.TopicArn = "arn:aws:sns:ap-south-1:999:someone-elses"
	if _, err := f.sns.Verify(context.Background(), f.sign(t, m)); !errors.Is(err, ErrSNSSignature) {
		t.Errorf("Verify error = %v, want ErrSNSSignature", err)
	}
}

func TestSNSPinsTheRegionHost(t *testing.T) {
	s := &SNS{Region: "ap-south-1"}
	if err := s.checkURL("https://sns.ap-south-1.amazonaws.com.evil.example/cert.pem"); err == nil {
		t.Error("a look-alike host was accepted")
	}
}

func TestSNSConfirmsOnThePinnedHostOnly(t *testing.T) {
	f := newSNSFixture(t)
	err := f.sns.ConfirmSubscription(context.Background(), SNSMessage{SubscribeURL: "https://example.com/confirm"})
	if !errors.Is(err, ErrSNSSignature) {
		t.Errorf("ConfirmSubscription error = %v, want ErrSNSSignature", err)
	}
}

func TestSuppressions(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    []Suppression
	}{
		{"permanent bounce", `{"notificationType":"Bounce","bounce":{"bounceType":"Permanent","bouncedRecipients":[{"emailAddress":"Guest@Example.com"}]}}`,
			[]Suppression{{"guest@example.com", "bounce"}}},
		{"transient bounce", `{"notificationType":"Bounce","bounce":{"bounceType":"Transient","bouncedRecipients":[{"emailAddress":"guest@example.com"}]}}`, nil},
		{"complaint event", `{"eventType":"Complaint","complaint":{"complainedRecipients":[{"emailAddress":"Emma <emma@example.com>"}]}}`,
			[]Suppression{{"emma@example.com", "complaint"}}},
		{"delivery", `{"notificationType":"Delivery"}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Suppressions(tt.message)
			if err != nil {
				t.Fatal(err)
			}
			if !equalSuppressions(got, tt.want) {
				t.Errorf("Suppressions = %v, want %v", got, tt.want)
			}
		})
	}
}

func equalSuppressions(a, b []Suppression) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type snsFixture struct {
	key     *rsa.PrivateKey
	certURL string
	sns     *SNS
}

// newSNSFixture serves a self-signed signing certificate over TLS and points an SNS checker at it.
func newSNSFixture(t *testing.T) *snsFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "sns.amazonaws.com"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(certPEM) }))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return &snsFixture{key: key, certURL: srv.URL + "/SimpleNotificationService-1.pem", sns: &SNS{Region: "ap-south-1", TopicARN: "arn:aws:sns:ap-south-1:123:ses", Host: u.Host, Client: srv.Client()}}
}

func (f *snsFixture) bounce() SNSMessage {
	return SNSMessage{
		Type: SNSNotification, MessageID: "m-1", TopicArn: "arn:aws:sns:ap-south-1:123:ses",
		Message:          `{"notificationType":"Bounce","bounce":{"bounceType":"Permanent","bouncedRecipients":[{"emailAddress":"guest@example.com"}]}}`,
		Timestamp:        "2026-10-18T05:00:00.000Z",
		SignatureVersion: "2", SigningCertURL: f.certURL,
	}
}

func (f *snsFixture) sign(t *testing.T, m SNSMessage) []byte {
	t.Helper()
	payload, err := stringToSign(m)
	if err != nil {
		t.Fatal(err)
	}
	hash, digest := crypto.SHA256, sha256.Sum256([]byte(payload))
	sum := digest[:]
	if m.SignatureVersion == "1" {
		hash = crypto.SHA1
		h := crypto.SHA1.New()
		h.Write([]byte(payload))
		sum = h.Sum(nil)
	}
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, hash, sum)
	if err != nil {
		t.Fatal(err)
	}
	m.Signature = base64.StdEncoding.EncodeToString(sig)
	body, _ := json.Marshal(m)
	return body
}
