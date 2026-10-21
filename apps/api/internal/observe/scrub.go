package observe

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/getsentry/sentry-go"
)

const removed = "[removed]"

// Personal data and secrets that must never leave in a report. The order matters: keys and
// tokens go before the looser number patterns.
var scrubbers = []*regexp.Regexp{
	regexp.MustCompile(`sk_(?:live|test)_[A-Za-z0-9_\-]+`),
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=\-]+`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`),
	regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	regexp.MustCompile(`\+\d[\d \-]{6,16}\d`),
	regexp.MustCompile(`\b0?7\d{8}\b`),
	regexp.MustCompile(`\b\d{9}[VvXx]\b`),
	regexp.MustCompile(`\b(?:19|20)\d{10}\b`),
}

// Scrub replaces phone numbers, email addresses, Payments.lk keys, bearer and JSON web tokens,
// and NIC numbers in s.
func Scrub(s string) string {
	for _, re := range scrubbers {
		s = re.ReplaceAllString(s, removed)
	}
	return s
}

// Path segments that are secrets in themselves.
var tokenPaths = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`/join/[^/]+`), "/join/:token"},
	{regexp.MustCompile(`/verify-email/[^/]+`), "/verify-email/:token"},
	{regexp.MustCompile(`/ical/[^/]+`), "/ical/:file"},
}

// CleanURL drops a URL's query and fragment, which can carry guest tokens, and replaces token
// path segments.
func CleanURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return removed
	}
	u.RawQuery, u.Fragment, u.User = "", "", nil
	path := u.Path
	for _, t := range tokenPaths {
		path = t.re.ReplaceAllString(path, t.with)
	}
	u.Path, u.RawPath = path, ""
	return Scrub(u.String())
}

// Clean strips an event down to what is safe to send: the request's method, cleaned URL and
// user agent; ids for the user; and scrubbed messages, exceptions, tags and contexts.
func Clean(e *sentry.Event) *sentry.Event {
	if e == nil {
		return nil
	}
	if r := e.Request; r != nil {
		agent := ""
		for k, v := range r.Headers {
			if strings.EqualFold(k, "User-Agent") {
				agent = v
			}
		}
		e.Request = &sentry.Request{Method: r.Method, URL: CleanURL(r.URL)}
		if agent != "" {
			e.Request.Headers = map[string]string{"User-Agent": agent}
		}
	}
	e.User = sentry.User{ID: e.User.ID}
	e.Message = Scrub(e.Message)
	for i := range e.Exception {
		e.Exception[i].Value = Scrub(e.Exception[i].Value)
	}
	for k, v := range e.Tags {
		e.Tags[k] = Scrub(v)
	}
	for name, c := range e.Contexts {
		for k, v := range c {
			if s, ok := v.(string); ok {
				c[k] = Scrub(s)
			}
		}
		e.Contexts[name] = c
	}
	kept := e.Breadcrumbs[:0]
	for _, b := range e.Breadcrumbs {
		if b = cleanBreadcrumb(b); b != nil {
			kept = append(kept, b)
		}
	}
	e.Breadcrumbs = kept
	return e
}

func cleanBreadcrumb(b *sentry.Breadcrumb) *sentry.Breadcrumb {
	if b == nil {
		return nil
	}
	b.Message = Scrub(b.Message)
	for k, v := range b.Data {
		switch s := v.(type) {
		case string:
			if k == "url" {
				b.Data[k] = CleanURL(s)
			} else {
				b.Data[k] = Scrub(s)
			}
		default:
			if k == "body" || k == "data" {
				delete(b.Data, k)
			}
		}
	}
	return b
}
