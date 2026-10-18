package email

import (
	"context"
	"errors"
	"fmt"
	"net/mail"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// SESConfig reaches SES v2 with explicit keys.
type SESConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	// From is an address on the verified domain, for example hello@mail.staykey.direct.
	From string
	// ConfigurationSet routes bounces and complaints to the SNS topic. Optional.
	ConfigurationSet string
	// Endpoint overrides the SES endpoint, for tests.
	Endpoint string
}

// SES sends email through Amazon SES v2.
type SES struct {
	client *sesv2.Client
	cfg    SESConfig
}

var _ Sender = (*SES)(nil)

// NewSES returns a sender for cfg.
func NewSES(cfg SESConfig) (*SES, error) {
	if cfg.Region == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.From == "" {
		return nil, errors.New("SES needs a region, access key, secret and from address")
	}
	if _, err := mail.ParseAddress(cfg.From); err != nil {
		return nil, fmt.Errorf("SES from address: %w", err)
	}
	opts := sesv2.Options{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	}
	if cfg.Endpoint != "" {
		opts.BaseEndpoint = aws.String(cfg.Endpoint)
	}
	return &SES{client: sesv2.New(opts), cfg: cfg}, nil
}

// Send sends m as a simple email with a text and an HTML part.
func (s *SES) Send(ctx context.Context, m Message) (string, error) {
	from := (&mail.Address{Name: m.FromName, Address: s.cfg.From}).String()
	in := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination:      &types.Destination{ToAddresses: []string{m.To}},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: aws.String(m.Subject), Charset: aws.String("UTF-8")},
			Body: &types.Body{
				Text: &types.Content{Data: aws.String(m.Text), Charset: aws.String("UTF-8")},
				Html: &types.Content{Data: aws.String(m.HTML), Charset: aws.String("UTF-8")},
			},
		}},
	}
	if m.ReplyTo != "" {
		in.ReplyToAddresses = []string{m.ReplyTo}
	}
	if s.cfg.ConfigurationSet != "" {
		in.ConfigurationSetName = aws.String(s.cfg.ConfigurationSet)
	}
	out, err := s.client.SendEmail(ctx, in)
	if err != nil {
		return "", fmt.Errorf("ses send: %w", err)
	}
	if out.MessageId == nil || *out.MessageId == "" {
		return "", errors.New("ses send: no message id in the response")
	}
	return *out.MessageId, nil
}
