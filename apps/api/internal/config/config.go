// Package config reads runtime settings from the environment.
package config

import (
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
}

const (
	devDatabaseURL = "postgres://staykey:staykey@localhost:5432/staykey?sslmode=disable"
	// devAuthSecret keeps development sessions valid across restarts. Never used in production.
	devAuthSecret = "development-only-secret-change-me-in-production"
)

// Load reads the configuration. Development gets working defaults that match
// docker-compose.yml; production requires DATABASE_URL, STAYKEY_AUTH_SECRET and
// STAYKEY_SMS_PROVIDER to be set explicitly.
func Load() (Config, error) {
	cfg := Config{
		Env:            getenv("APP_ENV", "development"),
		Port:           getenv("PORT", "8080"),
		ClientIPHeader: os.Getenv("STAYKEY_CLIENT_IP_HEADER"),
	}

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
		cfg.BookingDomain = getenv("STAYKEY_BOOKING_DOMAIN", "staykey.direct")
		cfg.BookingScheme = getenv("STAYKEY_BOOKING_SCHEME", "https")
		return cfg, cfg.validate()
	}

	cfg.DatabaseURL = getenv("DATABASE_URL", devDatabaseURL)
	cfg.AuthSecret = getenv("STAYKEY_AUTH_SECRET", devAuthSecret)
	cfg.SMSProvider = getenv("STAYKEY_SMS_PROVIDER", "log")
	cfg.BookingDomain = getenv("STAYKEY_BOOKING_DOMAIN", "localhost:3001")
	cfg.BookingScheme = getenv("STAYKEY_BOOKING_SCHEME", "http")
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if c.SMSProvider != "log" {
		return fmt.Errorf("unknown STAYKEY_SMS_PROVIDER %q (supported: log)", c.SMSProvider)
	}
	return nil
}

// IsProduction reports whether the API runs in production.
func (c Config) IsProduction() bool { return c.Env == "production" }

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
