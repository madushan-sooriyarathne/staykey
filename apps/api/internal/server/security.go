package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/tenant"
)

// Security scheme names in packages/api-spec/openapi.yaml.
const (
	schemeBearer  = "bearerAuth"
	schemeAccount = "accountHeader"
)

// requirement is what an operation needs before its handler runs.
type requirement struct {
	user    bool // a valid access token
	account bool // and an X-Account-Id the user is a member of
}

// requirementsFrom reads each operation's security from the contract, keyed by route pattern
// ("POST /v1/properties"), so the spec decides which routes are public, which need a signed-in
// user, and which need an account.
func requirementsFrom(spec *openapi3.T) (map[string]requirement, error) {
	out := map[string]requirement{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			security := spec.Security
			if op.Security != nil {
				security = *op.Security
			}
			if len(security) > 1 {
				return nil, fmt.Errorf("%s %s: alternative security requirements are not supported", method, path)
			}
			var r requirement
			for _, alt := range security {
				for name := range alt {
					switch name {
					case schemeBearer:
						r.user = true
					case schemeAccount:
						r.account = true
					default:
						return nil, fmt.Errorf("%s %s: unknown security scheme %q", method, path, name)
					}
				}
			}
			if r.account && !r.user {
				return nil, fmt.Errorf("%s %s: %s needs %s too", method, path, schemeAccount, schemeBearer)
			}
			out[method+" "+path] = r
		}
	}
	return out, nil
}

// secure authenticates the caller and resolves the account for routes that need them, before
// the request body is read. It runs inside the router, so r.Pattern names the matched route;
// a route missing from the contract is refused.
func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, ok := s.reqs[r.Pattern]
		if !ok {
			s.log.Error("route has no security requirement in the contract", "pattern", r.Pattern)
			writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong on our side.")
			return
		}
		if !req.user {
			next.ServeHTTP(w, r)
			return
		}
		ctx, err := s.authorize(r, req)
		if err != nil {
			if !writeAPIError(w, err) {
				s.log.Error("authorize request", "method", r.Method, "path", r.URL.Path, "error", err)
			}
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) authorize(r *http.Request, req requirement) (context.Context, error) {
	ctx := r.Context()
	token, ok := bearerToken(r)
	if !ok {
		return nil, errUnauthorized
	}
	p, err := s.auth.Authenticate(ctx, token)
	if errors.Is(err, auth.ErrUnauthenticated) {
		return nil, errUnauthorized
	}
	if err != nil {
		return nil, err
	}
	ctx = auth.WithPrincipal(ctx, p)
	if !req.account {
		return ctx, nil
	}

	t, err := tenant.Resolve(ctx, s.store, p.UserID, r.Header.Get(tenant.Header))
	switch {
	case errors.Is(err, tenant.ErrAccountRequired):
		return nil, &apiError{status: http.StatusBadRequest, code: "account_required", message: "Send the account to act on in X-Account-Id."}
	case errors.Is(err, tenant.ErrInvalidAccountID):
		return nil, &apiError{status: http.StatusBadRequest, code: "invalid_account_id", message: "X-Account-Id must be an account id."}
	case errors.Is(err, tenant.ErrNoAccess):
		return nil, errNoAccount
	case err != nil:
		return nil, err
	}
	return tenant.With(ctx, t), nil
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

// principal and currentTenant read what secure put in the context. Handlers only call them on
// operations whose contract requires it, so a missing value is a server bug.
func principal(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return auth.Principal{}, errors.New("no principal in context; is the operation missing bearerAuth?")
	}
	return p, nil
}

func currentTenant(ctx context.Context) (tenant.Tenant, error) {
	t, ok := tenant.From(ctx)
	if !ok {
		return tenant.Tenant{}, errors.New("no tenant in context; is the operation missing accountHeader?")
	}
	return t, nil
}

type clientIPKey struct{}

// withClientIP records the caller's address for rate limits. Behind a proxy, header names the
// header that carries the real client address (Fly-Client-IP on Fly.io); otherwise the
// connection's remote address is used.
func withClientIP(next http.Handler, header string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw string
		if header != "" {
			raw, _, _ = strings.Cut(r.Header.Get(header), ",")
		} else {
			raw, _, _ = net.SplitHostPort(r.RemoteAddr)
		}
		if ip, err := netip.ParseAddr(strings.TrimSpace(raw)); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip.Unmap()))
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(clientIPKey{}).(netip.Addr)
	return ip
}
