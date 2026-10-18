// Package email sends email through AWS SES and reads the bounces and complaints SES reports
// through SNS. Messages are written elsewhere; this package only delivers them.
package email

import (
	"context"
	"log/slog"
	"strings"
)

// Message is one email to one address.
type Message struct {
	To string
	// FromName is shown beside the sending address, for example "Kingfisher Villa via StayKey".
	FromName string
	// ReplyTo is where replies go, or empty for none.
	ReplyTo string
	Subject string
	Text    string
	HTML    string
	// Link is the email's main link (a confirmation or stay page link). Only the log sender in
	// development uses it, so links can be opened locally.
	Link string
}

// Sender delivers a message and returns the provider's message id.
type Sender interface {
	Send(ctx context.Context, m Message) (messageID string, err error)
}

// LogSender stands in for SES until it is set up. It records that an email was skipped with the
// address masked, never the subject or body, which carry names. With ShowLinks, for development
// only, it also logs the message's link.
type LogSender struct {
	Logger    *slog.Logger
	ShowLinks bool
}

// Send logs that the email wasn't sent and returns an empty message id.
func (l LogSender) Send(ctx context.Context, m Message) (string, error) {
	attrs := []any{"to", MaskAddress(m.To)}
	if l.ShowLinks && m.Link != "" {
		attrs = append(attrs, "link", m.Link)
	}
	l.Logger.InfoContext(ctx, "email not sent (log provider)", attrs...)
	return "", nil
}

// MaskAddress keeps an address's first letter and its domain, enough to tell people apart in a
// log: "nimal@example.com" becomes "n***@example.com".
func MaskAddress(address string) string {
	local, domain, ok := strings.Cut(address, "@")
	if !ok || local == "" {
		return "***"
	}
	return local[:1] + "***@" + domain
}
