package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"staykey.direct/api/internal/oapi"
)

// apiError is an error response raised outside a handler's typed responses, by the security
// middleware for example. The strict handler's error hook writes it.
type apiError struct {
	status     int
	code       string
	message    string
	retryAfter int
	field      string
	clash      *oapi.Clash
}

func (e *apiError) Error() string { return e.code + ": " + e.message }

var (
	errUnauthorized = &apiError{status: http.StatusUnauthorized, code: "unauthorized", message: "Sign in again to continue."}
	errNoAccount    = &apiError{status: http.StatusNotFound, code: "account_not_found", message: "That account doesn't exist or you're not a member of it."}
	errForbidden    = &apiError{status: http.StatusForbidden, code: "forbidden", message: "Your role doesn't allow this. Ask the account owner."}
)

// writeAPIError writes err as JSON when it is an apiError, and as a generic 500 otherwise. It
// reports whether err was an apiError.
func writeAPIError(w http.ResponseWriter, err error) bool {
	var e *apiError
	if !errors.As(err, &e) {
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong on our side.")
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	if e.status == http.StatusConflict {
		w.WriteHeader(e.status)
		_ = json.NewEncoder(w).Encode(oapi.ConflictError{Code: oapi.ConflictErrorCode(e.code), Message: e.message, Clash: e.clash})
		return true
	}
	body := oapi.Error{Code: e.code, Message: e.message, Field: optional(e.field)}
	if e.retryAfter > 0 {
		body.RetryAfter = &e.retryAfter
		w.Header().Set("Retry-After", strconv.Itoa(e.retryAfter))
	}
	w.WriteHeader(e.status)
	_ = json.NewEncoder(w).Encode(body)
	return true
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(oapi.Error{Code: code, Message: message})
}

func errBody(code, message string) oapi.Error { return oapi.Error{Code: code, Message: message} }

func fieldErr(code, field, message string) oapi.Error {
	return oapi.Error{Code: code, Message: message, Field: &field}
}
