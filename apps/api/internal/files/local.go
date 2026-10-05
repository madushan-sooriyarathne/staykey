package files

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Local keeps files in a folder and serves them from the API at /media/. Upload URLs are signed
// with a secret, so only the file the API presigned can be written. For development only.
type Local struct {
	Dir    string
	Secret []byte
	TTL    time.Duration
	Now    func() time.Time
}

// NewLocal returns a store in dir, creating it when needed.
func NewLocal(dir string, secret []byte) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Local{Dir: dir, Secret: secret, TTL: 15 * time.Minute, Now: time.Now}, nil
}

var _ Store = (*Local)(nil)

// MediaPrefix is the path the local store serves files under.
const MediaPrefix = "/media/"

// PresignUpload returns a signed PUT URL on the API.
func (l *Local) PresignUpload(ctx context.Context, key, contentType string, size int64) (Upload, error) {
	if !ValidKey(key) {
		return Upload{}, fmt.Errorf("invalid key %q", key)
	}
	expires := l.Now().Add(l.TTL).Truncate(time.Second)
	q := url.Values{}
	q.Set("expires", strconv.FormatInt(expires.Unix(), 10))
	q.Set("size", strconv.FormatInt(size, 10))
	q.Set("type", contentType)
	q.Set("signature", l.sign(key, expires.Unix(), size, contentType))
	return Upload{
		Key:       key,
		URL:       l.URL(ctx, key) + "?" + q.Encode(),
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: expires,
		PublicURL: l.URL(ctx, key),
	}, nil
}

// Exists reports whether the file is on disk.
func (l *Local) Exists(_ context.Context, key string) (bool, error) {
	if !ValidKey(key) {
		return false, nil
	}
	_, err := os.Stat(l.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// URL is the API address that serves the file.
func (l *Local) URL(ctx context.Context, key string) string {
	return baseURL(ctx, "http://localhost:8080") + MediaPrefix + key
}

// Handler serves GET for stored files and PUT for signed uploads under MediaPrefix.
func (l *Local) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len(MediaPrefix):]
		if !ValidKey(key) {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			w.Header().Set("Content-Type", ContentType(key))
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.ServeFile(w, r, l.path(key))
		case http.MethodPut:
			l.put(w, r, key)
		default:
			w.Header().Set("Allow", "GET, HEAD, PUT")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func (l *Local) put(w http.ResponseWriter, r *http.Request, key string) {
	q := r.URL.Query()
	expires, err1 := strconv.ParseInt(q.Get("expires"), 10, 64)
	size, err2 := strconv.ParseInt(q.Get("size"), 10, 64)
	contentType := q.Get("type")
	switch {
	case err1 != nil || err2 != nil ||
		!hmac.Equal([]byte(q.Get("signature")), []byte(l.sign(key, expires, size, contentType))):
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	case l.Now().Unix() > expires:
		http.Error(w, "upload link expired", http.StatusForbidden)
		return
	case r.Header.Get("Content-Type") != contentType:
		http.Error(w, "content type does not match the upload link", http.StatusBadRequest)
		return
	}

	path := l.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		http.Error(w, "storage unavailable", http.StatusInternalServerError)
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		http.Error(w, "storage unavailable", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, io.LimitReader(r.Body, size+1))
	closeErr := tmp.Close()
	switch {
	case err != nil || closeErr != nil:
		http.Error(w, "upload failed", http.StatusBadRequest)
	case n != size:
		http.Error(w, "file size does not match the upload link", http.StatusBadRequest)
	case os.Rename(tmp.Name(), path) != nil:
		http.Error(w, "storage unavailable", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (l *Local) sign(key string, expires, size int64, contentType string) string {
	m := hmac.New(sha256.New, l.Secret)
	fmt.Fprintf(m, "%s\n%d\n%d\n%s", key, expires, size, contentType)
	return hex.EncodeToString(m.Sum(nil))
}

func (l *Local) path(key string) string {
	return filepath.Join(l.Dir, filepath.FromSlash(key))
}

// WrittenBefore walks the folder for account files.
func (l *Local) WrittenBefore(_ context.Context, t time.Time) ([]string, error) {
	var keys []string
	err := filepath.WalkDir(l.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(l.Dir, path)
		if err != nil {
			return err
		}
		if key := filepath.ToSlash(rel); ValidKey(key) && info.ModTime().Before(t) {
			keys = append(keys, key)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	return keys, nil
}

// Delete removes the file from disk.
func (l *Local) Delete(_ context.Context, key string) error {
	if !ValidKey(key) {
		return fmt.Errorf("invalid key %q", key)
	}
	if err := os.Remove(l.path(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}
