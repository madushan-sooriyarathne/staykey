package server

import (
	"math"
	"net/http"
	"strings"
	"time"

	"staykey.direct/api/internal/auth"
)

// limit is how many requests a key may make per window.
type limit struct {
	name string
	n    int
	per  time.Duration
}

// Limits on the routes guests and owners call. Sign-in codes have their own limits in
// internal/auth, and webhooks come from the providers.
var (
	limitHolds    = limit{"holds", 10, 10 * time.Minute}
	limitBookings = limit{"bookings", 5, 10 * time.Minute}
	limitUploads  = limit{"uploads", 10, time.Hour}
	limitPublic   = limit{"public", 120, time.Minute}
	limitOwner    = limit{"owner", 300, time.Minute}
)

// limited counts each request against its route's limit and answers 429 past it. It runs after
// secure, so owner routes are counted per signed-in person and public routes per address.
func (s *Server) limited(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l, key, ok := s.limitFor(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		allowed, wait := s.limiter.Allow(l.name+"|"+key, l.n, l.per)
		if !allowed {
			writeAPIError(w, &apiError{
				status: http.StatusTooManyRequests, code: "rate_limited",
				message:    "Too many tries. Please wait a few minutes.",
				retryAfter: int(math.Ceil(wait.Seconds())),
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limitFor(r *http.Request) (limit, string, bool) {
	ip := "ip:" + clientIP(r.Context()).String()
	_, path, _ := strings.Cut(r.Pattern, " ")
	switch r.Pattern {
	case "POST /v1/public/properties/{slug}/holds":
		return limitHolds, ip, true
	case "POST /v1/public/properties/{slug}/bookings":
		return limitBookings, ip, true
	case "POST /v1/public/bookings/{bookingId}/uploads", "POST /v1/public/bookings/{bookingId}/slips":
		return limitUploads, "booking:" + r.PathValue("bookingId"), true
	}
	switch {
	case strings.HasPrefix(path, "/v1/public/"), strings.HasPrefix(path, "/ical/"):
		return limitPublic, ip, true
	case strings.HasPrefix(path, "/webhooks/"), strings.HasPrefix(path, "/v1/auth/"):
		return limit{}, "", false
	}
	if p, ok := auth.PrincipalFrom(r.Context()); ok {
		return limitOwner, "user:" + p.UserID.String(), true
	}
	return limit{}, "", false
}
