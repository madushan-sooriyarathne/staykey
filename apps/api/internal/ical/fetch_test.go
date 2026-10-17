package ical

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchSendsTheETagAndReadsNotModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
	}))
	defer srv.Close()

	_, _, notModified, err := NewFetcher(true).Fetch(context.Background(), srv.URL, `"v1"`)
	if err != nil || !notModified {
		t.Errorf("Fetch = notModified %v, %v; want not modified", notModified, err)
	}
}

func TestFetchReturnsTheNewETag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v2"`)
		_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
	}))
	defer srv.Close()

	_, etag, _, err := NewFetcher(true).Fetch(context.Background(), srv.URL, `"v1"`)
	if err != nil || etag != `"v2"` {
		t.Errorf("Fetch = etag %q, %v", etag, err)
	}
}

func TestFetchCapsTheSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", MaxBytes+1)))
	}))
	defer srv.Close()

	if _, _, _, err := NewFetcher(true).Fetch(context.Background(), srv.URL, ""); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Fetch error = %v, want ErrTooLarge", err)
	}
}

func TestFetchRefusesPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	if _, _, _, err := NewFetcher(false).Fetch(context.Background(), srv.URL, ""); !errors.Is(err, ErrPrivateAddress) {
		t.Errorf("Fetch error = %v, want ErrPrivateAddress", err)
	}
}

func TestFetchRefusesOtherSchemes(t *testing.T) {
	if _, _, _, err := NewFetcher(false).Fetch(context.Background(), "file:///etc/passwd", ""); !errors.Is(err, ErrBadLink) {
		t.Errorf("Fetch error = %v, want ErrBadLink", err)
	}
}

func TestProblemNeverRepeatsTheLink(t *testing.T) {
	_, _, _, err := NewFetcher(false).Fetch(context.Background(), "http://127.0.0.1:1/secret-token.ics", "")
	if msg := Problem(err); strings.Contains(msg, "secret-token") {
		t.Errorf("Problem = %q", msg)
	}
}
