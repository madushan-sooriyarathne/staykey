// Package files stores photos, logos and bank slips. The app uploads straight to storage with a presigned
// URL, then sends the key back; the API keeps keys, never the files themselves.
//
// Production uses Cloudflare R2 through its S3 API. Development uses a folder on disk served by
// the API itself, with the same presign-then-upload flow.
package files

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Kind is what a file is for. It decides the key prefix and the size limit.
type Kind string

const (
	KindPhoto Kind = "photo"
	KindLogo  Kind = "logo"
	// KindSlip is a bank transfer receipt a guest uploads from the booking page.
	KindSlip Kind = "slip"
)

// MaxBytes is the largest upload for each kind.
var MaxBytes = map[Kind]int64{KindPhoto: 15 << 20, KindLogo: 5 << 20, KindSlip: 10 << 20}

// extensions maps accepted content types to the key's file extension.
var extensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/heic": "heic",
}

// ContentTypes lists the accepted image types.
func ContentTypes() []string { return []string{"image/jpeg", "image/png", "image/webp", "image/heic"} }

// Upload tells the client how to send a file.
type Upload struct {
	Key       string
	URL       string // where to send the file
	Method    string // always PUT
	Headers   map[string]string
	ExpiresAt time.Time
	PublicURL string // where the file is served once uploaded
}

// Store is where files live.
type Store interface {
	// PresignUpload returns a short-lived URL that accepts exactly one file of this type and size.
	PresignUpload(ctx context.Context, key, contentType string, size int64) (Upload, error)
	// Exists reports whether a file was uploaded under key.
	Exists(ctx context.Context, key string) (bool, error)
	// URL is where a stored file is served.
	URL(ctx context.Context, key string) string
}

// NewKey returns a fresh key for an account's file. It fails for unsupported content types.
func NewKey(accountID uuid.UUID, kind Kind, contentType string) (string, error) {
	ext, ok := extensions[contentType]
	if !ok {
		return "", fmt.Errorf("unsupported content type %q", contentType)
	}
	return fmt.Sprintf("accounts/%s/%ss/%s.%s", accountID, kind, uuid.Must(uuid.NewV7()), ext), nil
}

var keyPattern = regexp.MustCompile(`^accounts/[0-9a-f-]{36}/(photo|logo|slip)s/[0-9a-f-]{36}\.(jpg|png|webp|heic)$`)

// ValidKey reports whether key has the shape NewKey produces. It also rules out path tricks.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// Belongs reports whether key is a file of this kind in this account.
func Belongs(key string, accountID uuid.UUID, kind Kind) bool {
	return ValidKey(key) && strings.HasPrefix(key, fmt.Sprintf("accounts/%s/%ss/", accountID, kind))
}

// ContentType returns the content type for a key's extension.
func ContentType(key string) string {
	for ct, ext := range extensions {
		if strings.HasSuffix(key, "."+ext) {
			return ct
		}
	}
	return "application/octet-stream"
}

type baseURLKey struct{}

// WithBaseURL records the address clients reach the API on, so the development store can hand
// out URLs a phone on the same network can open.
func WithBaseURL(ctx context.Context, base string) context.Context {
	return context.WithValue(ctx, baseURLKey{}, strings.TrimRight(base, "/"))
}

func baseURL(ctx context.Context, fallback string) string {
	if b, ok := ctx.Value(baseURLKey{}).(string); ok && b != "" {
		return b
	}
	return fallback
}
