package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/oapi"
)

// Handler returns the API's http.Handler with routing, error handling and middleware.
func (s *Server) Handler() http.Handler {
	strict := oapi.NewStrictHandlerWithOptions(s, nil, oapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			if !writeAPIError(w, err) {
				s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
			}
		},
	})

	mux := http.NewServeMux()
	if s.opts.Media != nil {
		mux.Handle(files.MediaPrefix, s.opts.Media)
	}
	routes := oapi.HandlerWithOptions(strict, oapi.StdHTTPServerOptions{
		BaseRouter: mux,
		// Runs after routing (so r.Pattern is set) and before the body is decoded.
		Middlewares: []oapi.MiddlewareFunc{s.secure},
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		},
	})

	handler := withBaseURL(withClientIP(routes, s.opts.ClientIPHeader), s.opts.PublicURL)
	return s.logRequests(s.recoverPanics(cors(handler, s.opts.AllowAllOrigins)))
}

// cors lets booking pages and the embed widget call /v1/public from any origin. With
// allowAll (development only) every route is open, so the Expo web preview can call the API.
func cors(next http.Handler, allowAll bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowAll || strings.HasPrefix(r.URL.Path, "/v1/public/") {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", "*")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Account-Id")
			h.Set("Access-Control-Expose-Headers", "Retry-After")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic", "value", v, "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong on our side.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.LogAttrs(r.Context(), slog.LevelInfo, "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
