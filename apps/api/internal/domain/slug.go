package domain

import (
	"errors"
	"regexp"
	"strings"
)

// A slug is the subdomain label of a hosted booking page, for example
// "kingfisher" in kingfisher.staykey.direct. It must be a valid DNS label.
var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`)

// reservedSlugs are subdomains StayKey uses itself or may need later.
var reservedSlugs = map[string]bool{
	"www": true, "api": true, "app": true, "cdn": true, "book": true, "admin": true,
	"mail": true, "help": true, "support": true, "status": true, "docs": true,
	"dashboard": true, "static": true, "assets": true, "embed": true, "widget": true,
	"staykey": true, "blog": true, "dev": true, "staging": true,
}

// IsReservedSlug reports whether slug is held back for StayKey's own use.
func IsReservedSlug(slug string) bool { return reservedSlugs[slug] }

// Slugify turns a property name into a slug candidate, for example
// "Kingfisher Villa" into "kingfisher-villa".
func Slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	return s
}

// ValidateSlug checks that slug can be used as a booking page address.
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return errors.New("use 3 to 40 lowercase letters, numbers or dashes, starting and ending with a letter or number")
	}
	if IsReservedSlug(slug) {
		return errors.New("this address is reserved, please choose another")
	}
	return nil
}
