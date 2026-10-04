// Package config reads runtime settings from the environment.
package config

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
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
	// SMSProvider sends sign-in codes. Only "log" exists so far: codes are written to the log.
	SMSProvider string
	// ClientIPHeader names the header a trusted proxy puts the client address in, for example
	// Fly-Client-IP. Empty uses the connection's address.
	ClientIPHeader string
	// PublicURL is the API's own address. Empty derives it from each request (development).
	PublicURL string

	// DataKey encrypts sensitive columns such as bank account numbers. 32 bytes.
	DataKey []byte

	// Storage is where photos and logos live: "r2" in production, "local" in development.
	Storage  string
	MediaDir string // for local storage
	R2       R2
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
		MediaDir:       getenv("STAYKEY_MEDIA_DIR", ".media"),
		R2: R2{
			Endpoint:        os.Getenv("STAYKEY_R2_ENDPOINT"),
			Bucket:          os.Getenv("STAYKEY_R2_BUCKET"),
			AccessKeyID:     os.Getenv("STAYKEY_R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("STAYKEY_R2_SECRET_ACCESS_KEY"),
			PublicURL:       os.Getenv("STAYKEY_MEDIA_URL"),
		},
	}
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
		return cfg, cfg.validate()
	}

	cfg.DatabaseURL = getenv("DATABASE_URL", devDatabaseURL)
	cfg.AuthSecret = getenv("STAYKEY_AUTH_SECRET", devAuthSecret)
	cfg.SMSProvider = getenv("STAYKEY_SMS_PROVIDER", "log")
	cfg.Storage = getenv("STAYKEY_STORAGE", "local")
	cfg.BookingDomain = getenv("STAYKEY_BOOKING_DOMAIN", "localhost:3001")
	cfg.BookingScheme = getenv("STAYKEY_BOOKING_SCHEME", "http")
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if c.SMSProvider != "log" {
		return fmt.Errorf("unknown STAYKEY_SMS_PROVIDER %q (supported: log)", c.SMSProvider)
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
