// Package config reads runtime settings from the environment.
package config

import (
	"errors"
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
}

const devDatabaseURL = "postgres://staykey:staykey@localhost:5432/staykey?sslmode=disable"

// Load reads the configuration. Development gets working defaults that match
// docker-compose.yml; production requires DATABASE_URL to be set explicitly.
func Load() (Config, error) {
	cfg := Config{
		Env:  getenv("APP_ENV", "development"),
		Port: getenv("PORT", "8080"),
	}

	if cfg.IsProduction() {
		cfg.DatabaseURL = os.Getenv("DATABASE_URL")
		if cfg.DatabaseURL == "" {
			return Config{}, errors.New("DATABASE_URL is required in production")
		}
		cfg.BookingDomain = getenv("STAYKEY_BOOKING_DOMAIN", "staykey.direct")
		cfg.BookingScheme = getenv("STAYKEY_BOOKING_SCHEME", "https")
		return cfg, nil
	}

	cfg.DatabaseURL = getenv("DATABASE_URL", devDatabaseURL)
	cfg.BookingDomain = getenv("STAYKEY_BOOKING_DOMAIN", "localhost:3001")
	cfg.BookingScheme = getenv("STAYKEY_BOOKING_SCHEME", "http")
	return cfg, nil
}

// IsProduction reports whether the API runs in production.
func (c Config) IsProduction() bool { return c.Env == "production" }

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
