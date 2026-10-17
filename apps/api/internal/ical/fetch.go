package ical

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// MaxBytes is the most of a feed that is read. A villa's calendar is a few kilobytes.
const MaxBytes = 5 << 20

var (
	// ErrPrivateAddress means a feed link pointed at a loopback, private or otherwise internal
	// address, which owner links may not reach.
	ErrPrivateAddress = errors.New("ical: link points to a private address")
	// ErrTooLarge means the feed is bigger than MaxBytes.
	ErrTooLarge = errors.New("ical: calendar is too large")
	// ErrBadLink means the link isn't an http, https or webcal URL.
	ErrBadLink = errors.New("ical: unsupported link")
)

// StatusError is a feed answering with an HTTP error.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("ical: calendar answered %d", e.Code) }

// Fetcher reads OTA calendars over HTTP.
type Fetcher struct {
	client *http.Client
}

// NewFetcher builds a fetcher. Unless allowPrivate, it refuses to connect to internal addresses,
// checked on the resolved address of every connection, redirects included. It never uses a proxy,
// since the proxy's address would pass the check in place of the feed's.
func NewFetcher(allowPrivate bool) *Fetcher {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if !allowPrivate {
		dialer.Control = refusePrivate
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
	}
	return &Fetcher{client: &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("ical: too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrBadLink
			}
			return nil
		},
	}}
}

// Fetch reads a feed. etag is the one from the last read; when the feed hasn't changed it
// returns notModified and no body.
func (f *Fetcher) Fetch(ctx context.Context, link, etag string) (body []byte, newETag string, notModified bool, err error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return nil, "", false, ErrBadLink
	}
	switch strings.ToLower(u.Scheme) {
	case "webcal", "webcals":
		u.Scheme = "https"
	case "http", "https":
	default:
		return nil, "", false, ErrBadLink
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", false, ErrBadLink
	}
	req.Header.Set("User-Agent", "StayKey/1.0 (+https://staykey.direct)")
	req.Header.Set("Accept", "text/calendar, */*;q=0.5")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	res, err := f.client.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusNotModified:
		return nil, etag, true, nil
	case res.StatusCode != http.StatusOK:
		return nil, "", false, &StatusError{Code: res.StatusCode}
	}
	body, err = io.ReadAll(io.LimitReader(res.Body, MaxBytes+1))
	if err != nil {
		return nil, "", false, fmt.Errorf("ical: read body: %w", err)
	}
	if len(body) > MaxBytes {
		return nil, "", false, ErrTooLarge
	}
	return body, res.Header.Get("ETag"), false, nil
}

// Problem is what the owner reads when a feed couldn't be read. It never repeats the link, which
// carries the channel's secret.
func Problem(err error) string {
	var status *StatusError
	var netErr net.Error
	switch {
	case errors.As(err, &status) && (status.Code == http.StatusNotFound || status.Code == http.StatusGone):
		return "The calendar link wasn't found. Copy it again from the channel."
	case errors.As(err, &status) && (status.Code == http.StatusUnauthorized || status.Code == http.StatusForbidden):
		return "The channel refused the calendar link. Copy it again from the channel."
	case errors.As(err, &status):
		return fmt.Sprintf("The channel answered with an error (%d). We'll keep trying.", status.Code)
	case errors.Is(err, ErrPrivateAddress):
		return "That link points to a private address."
	case errors.Is(err, ErrBadLink):
		return "That link isn't a calendar link."
	case errors.Is(err, ErrTooLarge):
		return "That calendar is too large to import."
	case errors.Is(err, ErrNotCalendar):
		return "That link didn't return a calendar."
	case errors.As(err, &netErr) && netErr.Timeout():
		return "The channel took too long to answer. We'll keep trying."
	default:
		return "We couldn't reach the calendar link. We'll keep trying."
	}
}

func refusePrivate(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return ErrPrivateAddress
	}
	ip := ap.Addr().Unmap()
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip) {
		return ErrPrivateAddress
	}
	return nil
}

// cgnat is the carrier-grade NAT range, which cloud hosts also use for internal services.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")
