// Package config reads runtime settings from the environment.
package config

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds the API's runtime settings.
type Config struct {
	Env         string // "development" or "production"
	Port        string
	DatabaseURL string

	// BookingDomain is the root domain for hosted booking pages: a property with
	// slug "kingfisher" is served at <scheme>://kingfisher.<BookingDomain>.
	BookingDomain string
	BookingScheme string

	// AuthSecret signs access tokens and keys sign-in code hashes. At least 32 bytes.
	AuthSecret string
	// SMSProvider sends sign-in codes to Sri Lankan numbers: "log" writes them to the log,
	// "ozone" texts them through oZone Sender. Other countries are not supported yet.
	SMSProvider string
	Ozone       Ozone
	// ClientIPHeader names the header a trusted proxy puts the client address in, for example
	// Fly-Client-IP. Empty uses the connection's address.
	ClientIPHeader string
	// PublicURL is the API's own address. Empty derives it from each request (development).
	PublicURL string
	// InviteURL is where invite links point, followed by the token.
	InviteURL string

	// ExpoAccessToken authorizes push notifications when the Expo project requires it.
	ExpoAccessToken string
	// WhatsAppProvider sends team alerts on WhatsApp: "log" writes them to the log, "cloud"
	// sends an approved template through the WhatsApp Cloud API.
	WhatsAppProvider string
	WhatsApp         WhatsApp

	// ICalAllowPrivate lets calendar links reach loopback and private addresses, for a feed
	// served from a laptop in development. Refused in production, where owner links must not
	// reach internal services.
	ICalAllowPrivate bool

	// RevenueCat and PaymentsLK are the billing providers. Each is optional: without one, the
	// routes that need it answer 503 billing_not_configured.
	RevenueCat RevenueCat
	PaymentsLK PaymentsLK
	// BillingURL is the web billing page on the marketing site. Payments.lk returns owners to
	// it, and to its /done page after paying.
	BillingURL string
	// ForwardSecret lets the booking page and the marketing site, which call the API from their
	// servers, pass on the visitor's address for rate limits.
	ForwardSecret string

	// EmailProvider sends email: "log" records that an email was skipped (the default, also in
	// production until SES is set up), "ses" sends it through Amazon SES.
	EmailProvider string
	SES           SES
	// VerifyEmailURL is the marketing site's email confirmation page, followed by the token.
	VerifyEmailURL string

	// DataKey encrypts sensitive columns such as bank account numbers. 32 bytes.
	DataKey []byte

	// Storage is where photos and logos live: "r2" in production, "local" in development.
	Storage  string
	MediaDir string // for local storage
	R2       R2
}

// RevenueCat reads App Store and Google Play plans and, with ExternalKey, records web purchases.
type RevenueCat struct {
	SecretKey   string
	WebhookAuth string
	ExternalKey string
}

// PaymentsLK takes web payments. SecretKey is sk_test_... in the sandbox, sk_live_... live.
type PaymentsLK struct {
	SecretKey     string
	WebhookSecret string
	APIURL        string
}

// SES holds the Amazon SES v2 settings. From is an address on the verified domain;
// ConfigurationSet routes bounces and complaints to the SNS topic.
type SES struct {
	Region           string
	AccessKeyID      string
	SecretAccessKey  string
	ConfigurationSet string
	From             string
}

// WhatsApp holds the WhatsApp Cloud API settings for team alerts.
type WhatsApp struct {
	Token         string
	PhoneNumberID string
	Template      string
}

// Ozone holds the oZone Sender credentials from its documentation page.
type Ozone struct {
	UserID   string
	APIKey   string
	SenderID string
}

// R2 reaches the Cloudflare R2 bucket for photos and logos.
type R2 struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicURL       string
}

const (
	devDatabaseURL = "postgres://staykey:staykey@localhost:5432/staykey?sslmode=disable"
	// devAuthSecret keeps development sessions valid across restarts. Never used in production.
	devAuthSecret = "development-only-secret-change-me-in-production"
	// devDataKey derives the development encryption key. Never used in production.
	devDataKey = "development-only-data-key"
)

// Load reads the configuration. Development gets working defaults that match
// docker-compose.yml; production requires DATABASE_URL, STAYKEY_AUTH_SECRET,
// STAYKEY_DATA_KEY, STAYKEY_SMS_PROVIDER and the R2 settings to be set explicitly.
func Load() (Config, error) {
	cfg := Config{
		Env:            getenv("APP_ENV", "development"),
		Port:           getenv("PORT", "8080"),
		ClientIPHeader: os.Getenv("STAYKEY_CLIENT_IP_HEADER"),
		PublicURL:      os.Getenv("STAYKEY_PUBLIC_URL"),
		InviteURL:      getenv("STAYKEY_INVITE_URL", "https://staykey.direct/join"),

		ExpoAccessToken:  os.Getenv("STAYKEY_EXPO_ACCESS_TOKEN"),
		WhatsAppProvider: getenv("STAYKEY_WHATSAPP_PROVIDER", "log"),
		WhatsApp: WhatsApp{
			Token:         os.Getenv("STAYKEY_WHATSAPP_TOKEN"),
			PhoneNumberID: os.Getenv("STAYKEY_WHATSAPP_PHONE_NUMBER_ID"),
			Template:      os.Getenv("STAYKEY_WHATSAPP_TEMPLATE"),
		},
		EmailProvider: getenv("STAYKEY_EMAIL_PROVIDER", "log"),
		SES: SES{
			Region:           os.Getenv("STAYKEY_SES_REGION"),
			AccessKeyID:      os.Getenv("STAYKEY_SES_ACCESS_KEY_ID"),
			SecretAccessKey:  os.Getenv("STAYKEY_SES_SECRET_ACCESS_KEY"),
			ConfigurationSet: os.Getenv("STAYKEY_SES_CONFIGURATION_SET"),
			From:             os.Getenv("STAYKEY_EMAIL_FROM"),
		},
		MediaDir: getenv("STAYKEY_MEDIA_DIR", ".media"),
		Ozone: Ozone{
			UserID:   os.Getenv("STAYKEY_OZONE_USER_ID"),
			APIKey:   os.Getenv("STAYKEY_OZONE_API_KEY"),
			SenderID: os.Getenv("STAYKEY_OZONE_SENDER_ID"),
		},
		RevenueCat: RevenueCat{
			SecretKey:   os.Getenv("STAYKEY_REVENUECAT_SECRET_KEY"),
			WebhookAuth: os.Getenv("STAYKEY_REVENUECAT_WEBHOOK_AUTH"),
			ExternalKey: os.Getenv("STAYKEY_REVENUECAT_EXTERNAL_KEY"),
		},
		PaymentsLK: PaymentsLK{
			SecretKey:     os.Getenv("STAYKEY_PAYMENTSLK_SECRET_KEY"),
			WebhookSecret: os.Getenv("STAYKEY_PAYMENTSLK_WEBHOOK_SECRET"),
			APIURL:        os.Getenv("STAYKEY_PAYMENTSLK_API_URL"),
		},
		ForwardSecret: os.Getenv("STAYKEY_FORWARD_SECRET"),
		R2: R2{
			Endpoint:        os.Getenv("STAYKEY_R2_ENDPOINT"),
			Bucket:          os.Getenv("STAYKEY_R2_BUCKET"),
			AccessKeyID:     os.Getenv("STAYKEY_R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("STAYKEY_R2_SECRET_ACCESS_KEY"),
			PublicURL:       os.Getenv("STAYKEY_MEDIA_URL"),
		},
	}
	allowPrivate := getenv("STAYKEY_ICAL_ALLOW_PRIVATE", "")
	if allowPrivate != "" && allowPrivate != "true" && allowPrivate != "false" {
		return Config{}, errors.New("STAYKEY_ICAL_ALLOW_PRIVATE must be true or false")
	}
	if cfg.IsProduction() && allowPrivate == "true" {
		return Config{}, errors.New("STAYKEY_ICAL_ALLOW_PRIVATE can't be true in production")
	}
	cfg.ICalAllowPrivate = !cfg.IsProduction() && allowPrivate != "false"

	key, err := dataKey(cfg.IsProduction())
	if err != nil {
		return Config{}, err
	}
	cfg.DataKey = key

	if cfg.IsProduction() {
		cfg.DatabaseURL = os.Getenv("DATABASE_URL")
		cfg.AuthSecret = os.Getenv("STAYKEY_AUTH_SECRET")
		cfg.SMSProvider = os.Getenv("STAYKEY_SMS_PROVIDER")
		switch {
		case cfg.DatabaseURL == "":
			return Config{}, errors.New("DATABASE_URL is required in production")
		case len(cfg.AuthSecret) < 32:
			return Config{}, errors.New("STAYKEY_AUTH_SECRET of at least 32 bytes is required in production")
		case cfg.SMSProvider == "":
			return Config{}, errors.New("STAYKEY_SMS_PROVIDER is required in production")
		}
		cfg.Storage = getenv("STAYKEY_STORAGE", "r2")
		cfg.BookingDomain = getenv("STAYKEY_BOOKING_DOMAIN", "staykey.direct")
		cfg.BookingScheme = getenv("STAYKEY_BOOKING_SCHEME", "https")
		cfg.BillingURL = getenv("STAYKEY_BILLING_URL", "https://staykey.direct/billing")
		cfg.VerifyEmailURL = getenv("STAYKEY_VERIFY_EMAIL_URL", "https://staykey.direct/verify-email")
		return cfg, cfg.validate()
	}

	cfg.DatabaseURL = getenv("DATABASE_URL", devDatabaseURL)
	cfg.AuthSecret = getenv("STAYKEY_AUTH_SECRET", devAuthSecret)
	cfg.SMSProvider = getenv("STAYKEY_SMS_PROVIDER", "log")
	cfg.Storage = getenv("STAYKEY_STORAGE", "local")
	cfg.BookingDomain = getenv("STAYKEY_BOOKING_DOMAIN", "localhost:3001")
	cfg.BookingScheme = getenv("STAYKEY_BOOKING_SCHEME", "http")
	cfg.BillingURL = getenv("STAYKEY_BILLING_URL", "http://localhost:3000/billing")
	cfg.VerifyEmailURL = getenv("STAYKEY_VERIFY_EMAIL_URL", "http://localhost:3000/verify-email")
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	switch c.SMSProvider {
	case "log":
	case "ozone":
		o := c.Ozone
		if o.UserID == "" || o.APIKey == "" || o.SenderID == "" {
			return errors.New("ozone sms needs STAYKEY_OZONE_USER_ID, STAYKEY_OZONE_API_KEY and STAYKEY_OZONE_SENDER_ID")
		}
	default:
		return fmt.Errorf("unknown STAYKEY_SMS_PROVIDER %q (supported: log, ozone)", c.SMSProvider)
	}
	switch c.WhatsAppProvider {
	case "log":
	case "cloud":
		w := c.WhatsApp
		if w.Token == "" || w.PhoneNumberID == "" || w.Template == "" {
			return errors.New("cloud whatsapp needs STAYKEY_WHATSAPP_TOKEN, STAYKEY_WHATSAPP_PHONE_NUMBER_ID and STAYKEY_WHATSAPP_TEMPLATE")
		}
	default:
		return fmt.Errorf("unknown STAYKEY_WHATSAPP_PROVIDER %q (supported: log, cloud)", c.WhatsAppProvider)
	}
	switch c.EmailProvider {
	case "log":
	case "ses":
		e := c.SES
		if e.Region == "" || e.AccessKeyID == "" || e.SecretAccessKey == "" || e.From == "" {
			return errors.New("ses email needs STAYKEY_SES_REGION, STAYKEY_SES_ACCESS_KEY_ID, STAYKEY_SES_SECRET_ACCESS_KEY and STAYKEY_EMAIL_FROM")
		}
	default:
		return fmt.Errorf("unknown STAYKEY_EMAIL_PROVIDER %q (supported: log, ses)", c.EmailProvider)
	}
	if p := c.PaymentsLK; p.SecretKey != "" {
		if p.WebhookSecret == "" {
			return errors.New("payments.lk needs STAYKEY_PAYMENTSLK_WEBHOOK_SECRET with STAYKEY_PAYMENTSLK_SECRET_KEY")
		}
		if c.IsProduction() && !strings.HasPrefix(p.SecretKey, "sk_live_") {
			return errors.New("STAYKEY_PAYMENTSLK_SECRET_KEY must be a live key (sk_live_...) in production")
		}
	}
	if c.RevenueCat.WebhookAuth != "" && c.RevenueCat.SecretKey == "" {
		return errors.New("the RevenueCat webhook needs STAYKEY_REVENUECAT_SECRET_KEY to read subscribers")
	}
	if c.ForwardSecret != "" && len(c.ForwardSecret) < 32 {
		return errors.New("STAYKEY_FORWARD_SECRET must be at least 32 bytes: openssl rand -base64 32")
	}
	switch c.Storage {
	case "local":
		if c.IsProduction() {
			return errors.New("STAYKEY_STORAGE=local is for development; use r2 in production")
		}
	case "r2":
		r := c.R2
		if r.Endpoint == "" || r.Bucket == "" || r.AccessKeyID == "" || r.SecretAccessKey == "" || r.PublicURL == "" {
			return errors.New("R2 storage needs STAYKEY_R2_ENDPOINT, STAYKEY_R2_BUCKET, STAYKEY_R2_ACCESS_KEY_ID, STAYKEY_R2_SECRET_ACCESS_KEY and STAYKEY_MEDIA_URL")
		}
	default:
		return fmt.Errorf("unknown STAYKEY_STORAGE %q (supported: local, r2)", c.Storage)
	}
	return nil
}

// dataKey reads STAYKEY_DATA_KEY, 32 bytes in base64. Development falls back to a fixed key.
func dataKey(production bool) ([]byte, error) {
	raw := os.Getenv("STAYKEY_DATA_KEY")
	if raw == "" {
		if production {
			return nil, errors.New("STAYKEY_DATA_KEY (32 bytes, base64) is required in production")
		}
		key := sha256.Sum256([]byte(devDataKey))
		return key[:], nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, errors.New("STAYKEY_DATA_KEY must be 32 bytes in base64: openssl rand -base64 32")
	}
	return key, nil
}

// IsProduction reports whether the API runs in production.
func (c Config) IsProduction() bool { return c.Env == "production" }

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
