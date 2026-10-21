package observe

import (
	"context"
	"net/http"
	"regexp"

	"github.com/google/uuid"
)

// HeaderRequestID carries a request's id in and out.
const HeaderRequestID = "X-Request-Id"

type requestIDKey struct{}

var validID = regexp.MustCompile(`^[A-Za-z0-9\-]{8,64}$`)

// RequestIDs gives every request an id, on its context and on the response. An incoming id is
// kept only when trusted says the caller may set one (StayKey's own sites), so outsiders can't
// choose ids that collide with real ones.
func RequestIDs(next http.Handler, trusted func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !validID.MatchString(id) || trusted == nil || !trusted(r) {
			id = newID()
		}
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}

// WithRequestID puts a request id on ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID is the id of the request ctx belongs to, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func newID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	return uuid.NewString()
}
