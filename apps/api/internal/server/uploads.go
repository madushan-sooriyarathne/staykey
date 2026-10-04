package server

import (
	"context"
	"fmt"
	"net/http"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/oapi"
)

// CreateUpload presigns an upload for a photo or logo in the caller's account.
func (s *Server) CreateUpload(ctx context.Context, req oapi.CreateUploadRequestObject) (oapi.CreateUploadResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermSettings) {
		return nil, errForbidden
	}
	if req.Body == nil {
		return oapi.CreateUpload400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(errBody("invalid_body", "Request body is required."))}, nil
	}
	kind := files.Kind(req.Body.Kind)
	limit, ok := files.MaxBytes[kind]
	if !ok {
		return oapi.CreateUpload400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
			fieldErr("invalid_kind", "kind", "Uploads are photos or logos."))}, nil
	}
	if req.Body.Size < 1 || req.Body.Size > limit {
		return oapi.CreateUpload400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
			fieldErr("file_too_large", "size", fmt.Sprintf("Files can be up to %d MB.", limit>>20)))}, nil
	}
	key, err := files.NewKey(t.AccountID, kind, string(req.Body.ContentType))
	if err != nil {
		return oapi.CreateUpload400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
			fieldErr("invalid_content_type", "contentType", "Use a JPEG, PNG, WebP or HEIC image."))}, nil
	}
	up, err := s.files.PresignUpload(ctx, key, string(req.Body.ContentType), req.Body.Size)
	if err != nil {
		return nil, err
	}
	return oapi.CreateUpload201JSONResponse{
		Key: up.Key, UploadUrl: up.URL, Method: oapi.PUT, Headers: up.Headers,
		ExpiresAt: up.ExpiresAt, Url: up.PublicURL,
	}, nil
}

// withBaseURL records how this request reached the API, so the development file store builds
// URLs that work from a phone on the same network.
func withBaseURL(next http.Handler, fixed string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := fixed
		if base == "" {
			scheme := "http"
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			base = scheme + "://" + r.Host
		}
		next.ServeHTTP(w, r.WithContext(files.WithBaseURL(r.Context(), base)))
	})
}
