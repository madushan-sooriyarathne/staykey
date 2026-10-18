package server

import (
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
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"staykey.direct/api/internal/email"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/store"
)

// snsSigner stands in for SNS: it serves a signing certificate over TLS and signs messages
// with its key.
type snsSigner struct {
	key     *rsa.PrivateKey
	certURL string
}

func withSNS(t *testing.T) (func(*Options), *snsSigner) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "sns"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(certPEM) }))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	set := func(o *Options) { o.SNS = &email.SNS{Region: "ap-south-1", Host: u.Host, Client: srv.Client()} }
	return set, &snsSigner{key: key, certURL: srv.URL + "/cert.pem"}
}

// bounce is a signed SNS notification of a permanent bounce for address, signed with key.
func (sg *snsSigner) bounce(t *testing.T, address string, key *rsa.PrivateKey) []byte {
	t.Helper()
	m := map[string]string{
		"Type": "Notification", "MessageId": "m-1", "TopicArn": "arn:aws:sns:ap-south-1:1:ses",
		"Message":   `{"notificationType":"Bounce","bounce":{"bounceType":"Permanent","bouncedRecipients":[{"emailAddress":"` + address + `"}]}}`,
		"Timestamp": "2026-10-18T05:00:00.000Z", "SignatureVersion": "2", "SigningCertURL": sg.certURL,
	}
	payload := "Message\n" + m["Message"] + "\nMessageId\n" + m["MessageId"] + "\nTimestamp\n" + m["Timestamp"] +
		"\nTopicArn\n" + m["TopicArn"] + "\nType\n" + m["Type"] + "\n"
	digest := sha256.Sum256([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	m["Signature"] = base64.StdEncoding.EncodeToString(sig)
	body, _ := json.Marshal(m)
	return body
}

func (e *env) suppressed(address string) bool {
	e.t.Helper()
	var n int
	if err := e.store.Pool().QueryRow(context.Background(), "SELECT count(*) FROM email_suppressions WHERE address = $1", address).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n > 0
}

func TestAForgedSNSMessageIsRefused(t *testing.T) {
	t.Parallel()
	set, sg := withSNS(t)
	e := newEnvWithOptions(t, set)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	rec := e.do(call{method: http.MethodPost, path: "/webhooks/ses", raw: sg.bounce(t, "guest@staykey.direct", other)})
	e.expect(rec, http.StatusBadRequest)
}

func TestAForgedSNSMessageSuppressesNothing(t *testing.T) {
	t.Parallel()
	set, sg := withSNS(t)
	e := newEnvWithOptions(t, set)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	e.do(call{method: http.MethodPost, path: "/webhooks/ses", raw: sg.bounce(t, "guest@staykey.direct", other)})
	if e.suppressed("guest@staykey.direct") {
		t.Error("a forged bounce suppressed the address")
	}
}

func TestASignedBounceSuppressesTheAddress(t *testing.T) {
	t.Parallel()
	set, sg := withSNS(t)
	e := newEnvWithOptions(t, set)
	rec := e.do(call{method: http.MethodPost, path: "/webhooks/ses", raw: sg.bounce(t, "guest@staykey.direct", sg.key)})
	e.expect(rec, http.StatusOK)
	if !e.suppressed("guest@staykey.direct") {
		t.Error("the bounced address isn't suppressed")
	}
}

func TestTheSESWebhookIsOffWithoutSNS(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rec := e.do(call{method: http.MethodPost, path: "/webhooks/ses", raw: []byte("{}")})
	e.expect(rec, http.StatusServiceUnavailable)
}

// addEmail sets the person's email and returns the token from the confirmation link sent.
func (e *env) addEmail(s *session, address string) string {
	e.t.Helper()
	e.expect(e.do(call{method: http.MethodPatch, path: "/v1/me", body: map[string]any{"email": address}, as: s}), http.StatusOK)
	token := ""
	if _, err := e.store.SendEmails(context.Background(), 50, time.Now(), func(_ context.Context, m store.Email) (string, error) {
		if m.Kind.CarriesConfirmation() {
			token = m.Link[strings.LastIndex(m.Link, "/")+1:]
		}
		return "msg", nil
	}); err != nil {
		e.t.Fatal(err)
	}
	if token == "" {
		e.t.Fatal("no confirmation email")
	}
	return token
}

func TestConfirmingThroughThePublicRouteConfirmsTheEmail(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.signIn("+94776000001")
	token := e.addEmail(s, "nimal@staykey.direct")
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/public/email-confirmation", body: map[string]string{"token": token}}), http.StatusOK)
	rec := e.do(call{method: http.MethodGet, path: "/v1/me", as: s})
	e.expect(rec, http.StatusOK)
	if me := decode[oapi.Me](t, rec); !me.User.EmailConfirmed {
		t.Error("emailConfirmed = false after confirming")
	}
}

func TestAnUnknownConfirmationTokenIsNotFound(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rec := e.do(call{method: http.MethodPost, path: "/v1/public/email-confirmation", body: map[string]string{"token": "nope"}})
	e.expect(rec, http.StatusNotFound)
}

func TestResendingAConfirmedEmailIsAConflict(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.signIn("+94776000002")
	token := e.addEmail(s, "nimal@staykey.direct")
	e.do(call{method: http.MethodPost, path: "/v1/public/email-confirmation", body: map[string]string{"token": token}})
	rec := e.do(call{method: http.MethodPost, path: "/v1/me/email-confirmation", as: s})
	e.expect(rec, http.StatusConflict)
}

func TestResendingWithoutAnEmailIsABadRequest(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.signIn("+94776000003")
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/me/email-confirmation", as: s}), http.StatusBadRequest)
}

func TestResendingIsLimitedToThreeAnHour(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.signIn("+94776000004")
	e.addEmail(s, "nimal@staykey.direct")
	for range 3 {
		e.expect(e.do(call{method: http.MethodPost, path: "/v1/me/email-confirmation", as: s}), http.StatusNoContent)
	}
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/me/email-confirmation", as: s}), http.StatusTooManyRequests)
}

func TestTemplateChannelsCanTurnEmailOff(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94776000005", "Template account")
	rec := e.do(call{method: http.MethodGet, path: "/v1/templates", as: s, account: account})
	e.expect(rec, http.StatusOK)
	id := decode[oapi.TemplateList](t, rec).Items[0].Id
	rec = e.do(call{method: http.MethodPatch, path: "/v1/templates/" + id.String(), body: map[string]any{"channels": []string{"whatsapp"}}, as: s, account: account})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.Template](t, rec).Channels; len(got) != 1 || got[0] != oapi.TemplateChannelWhatsapp {
		t.Errorf("channels = %v, want [whatsapp]", got)
	}
}

func TestAnUnknownTemplateChannelIsRefused(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94776000006", "Template account")
	rec := e.do(call{method: http.MethodGet, path: "/v1/templates", as: s, account: account})
	id := decode[oapi.TemplateList](t, rec).Items[0].Id
	rec = e.do(call{method: http.MethodPatch, path: "/v1/templates/" + id.String(), body: map[string]any{"channels": []string{"sms"}}, as: s, account: account})
	e.expect(rec, http.StatusBadRequest)
}
