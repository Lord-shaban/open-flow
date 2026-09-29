package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"github.com/Lord-shaban/open-flow/services/api/internal/studio"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func NewStudio(logger *slog.Logger, s *studio.Service) http.Handler {
	mux := http.NewServeMux()
	foundation := New(logger)
	mux.Handle("GET /healthz", foundation)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.Ready(r.Context()) != nil {
			writeJSON(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready", "scope": "image-studio"})
	})
	mux.HandleFunc("GET /v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"service": "open-flow", "version": "0.2.0", "stage": "M1", "capabilities": []string{"image_generation"}, "cost_policy": "free-providers-only", "video_generation": false})
	})
	protected := http.NewServeMux()
	protected.HandleFunc("GET /v1/credentials", func(w http.ResponseWriter, r *http.Request) {
		cs, err := s.Store.Credentials(r.Context(), s.OwnerID)
		respond(w, r, cs, err)
	})
	protected.HandleFunc("POST /v1/credentials", func(w http.ResponseWriter, r *http.Request) {
		var in studio.CredentialInput
		if !decode(w, r, &in) {
			return
		}
		c, err := s.SaveCredential(r.Context(), "", in)
		if err != nil {
			respond(w, r, nil, err)
			return
		}
		writeJSON(w, 201, c)
	})
	protected.HandleFunc("PUT /v1/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in studio.CredentialInput
		if !decode(w, r, &in) {
			return
		}
		c, err := s.SaveCredential(r.Context(), r.PathValue("id"), in)
		respond(w, r, c, err)
	})
	protected.HandleFunc("DELETE /v1/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := s.Store.RevokeCredential(r.Context(), s.OwnerID, r.PathValue("id"))
		respond(w, r, map[string]bool{"revoked": true}, err)
	})
	protected.HandleFunc("POST /v1/credentials/{id}/test", func(w http.ResponseWriter, r *http.Request) {
		c, err := s.Store.Credential(r.Context(), s.OwnerID, r.PathValue("id"))
		if err != nil || c.Status != "active" {
			respond(w, r, nil, persistence.ErrNotFound)
			return
		}
		_, err = s.Resolve(r.Context(), c.ID)
		if err != nil {
			respond(w, r, nil, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		adapter := s.Adapters[c.Provider]
		if adapter == nil {
			respond(w, r, nil, persistence.ErrInvalid)
			return
		}
		health, err := adapter.TestConnection(ctx, c.ID)
		respond(w, r, map[string]any{"available": health.Available, "checked_at": health.CheckedAt, "latency_ms": health.Latency.Milliseconds(), "test": "authentication-and-discovery", "generation_test": false}, err)
	})
	protected.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		models, err := s.Models(r.Context(), r.URL.Query().Get("provider"), r.URL.Query().Get("credential_id"))
		respond(w, r, map[string]any{"models": models, "verified_at": time.Now().UTC()}, err)
	})
	protected.HandleFunc("POST /v1/generations", func(w http.ResponseWriter, r *http.Request) {
		var in studio.GenerationInput
		if !decode(w, r, &in) {
			return
		}
		g, err := s.Generate(r.Context(), in, r.Header.Get("Idempotency-Key"))
		if err != nil {
			respond(w, r, nil, err)
			return
		}
		w.Header().Set("Location", "/v1/generations/"+g.ID)
		writeJSON(w, 202, g)
	})
	protected.HandleFunc("GET /v1/generations", func(w http.ResponseWriter, r *http.Request) {
		limit := 24
		if raw := r.URL.Query().Get("limit"); raw != "" {
			var err error
			limit, err = strconv.Atoi(raw)
			if err != nil {
				respond(w, r, nil, persistence.ErrInvalid)
				return
			}
		}
		gs, next, err := s.Store.Generations(r.Context(), s.OwnerID, r.URL.Query().Get("cursor"), limit)
		respond(w, r, map[string]any{"generations": gs, "next_cursor": next}, err)
	})
	protected.HandleFunc("GET /v1/generations/{id}", func(w http.ResponseWriter, r *http.Request) {
		g, err := s.Store.Generation(r.Context(), s.OwnerID, r.PathValue("id"))
		respond(w, r, g, err)
	})
	protected.HandleFunc("DELETE /v1/generations/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := s.Store.DeleteGeneration(r.Context(), s.OwnerID, r.PathValue("id"))
		respond(w, r, map[string]bool{"deleted": true}, err)
	})
	protected.HandleFunc("GET /v1/artifacts/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		artifact, err := s.Store.Artifact(r.Context(), s.OwnerID, r.PathValue("id"))
		if err != nil {
			respond(w, r, nil, err)
			return
		}
		expires := strconv.FormatInt(time.Now().Add(2*time.Minute).Unix(), 10)
		sig := downloadSignature(s.OwnerToken, s.OwnerID, artifact.ID, expires)
		writeJSON(w, 200, map[string]any{"url": "/v1/artifacts/" + artifact.ID + "/content?expires=" + expires + "&signature=" + sig, "expires_at": expires, "content_type": artifact.ContentType})
	})
	protected.HandleFunc("GET /v1/artifacts/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		expires := r.URL.Query().Get("expires")
		stamp, err := strconv.ParseInt(expires, 10, 64)
		if err != nil || stamp <= time.Now().Unix() || stamp > time.Now().Add(2*time.Minute).Unix() || !hmac.Equal([]byte(r.URL.Query().Get("signature")), []byte(downloadSignature(s.OwnerToken, s.OwnerID, id, expires))) {
			respond(w, r, nil, persistence.ErrNotFound)
			return
		}
		a, err := s.Store.Artifact(r.Context(), s.OwnerID, id)
		if err != nil {
			respond(w, r, nil, err)
			return
		}
		reader, err := s.Storage.Get(r.Context(), a.Key)
		if err != nil {
			respond(w, r, nil, err)
			return
		}
		defer reader.Close()
		w.Header().Set("Content-Type", a.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(a.Bytes, 10))
		extension := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp"}[a.ContentType]
		w.Header().Set("Content-Disposition", `inline; filename="open-flow-`+a.ID+`.`+extension+`"`)
		_, _ = io.CopyN(w, reader, a.Bytes)
	})
	mux.Handle("/v1/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash := sha256.Sum256([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
		expected := sha256.Sum256([]byte(s.OwnerToken))
		if s.OwnerToken == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || subtle.ConstantTimeCompare(hash[:], expected[:]) != 1 {
			writeJSON(w, 401, apiError("unauthorized", "Owner access is required", w.Header().Get("X-Request-ID")))
			return
		}
		protected.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("X-Request-ID", rand.Text())
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
		logger.Info("studio_request", "request_id", w.Header().Get("X-Request-ID"), "route", "/v1/*", "duration_ms", time.Since(start).Milliseconds())
	})
}
func downloadSignature(key, owner, id, expires string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte("download:v1:" + owner + ":" + id + ":" + expires))
	return hex.EncodeToString(mac.Sum(nil))
}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, 415, apiError("unsupported_media_type", "Use application/json", w.Header().Get("X-Request-ID")))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(out)
	var extra any
	if err != nil || decoder.Decode(&extra) != io.EOF {
		writeJSON(w, 400, apiError("invalid_request", "Invalid or oversized JSON body", w.Header().Get("X-Request-ID")))
		return false
	}
	return true
}
func respond(w http.ResponseWriter, r *http.Request, payload any, err error) {
	if err == nil {
		writeJSON(w, 200, payload)
		return
	}
	status, code, message := 500, "internal_error", "Request could not be completed"
	switch {
	case errors.Is(err, persistence.ErrInvalid):
		status, code, message = 400, "invalid_request", "Check the request fields"
	case errors.Is(err, persistence.ErrNotFound):
		status, code, message = 404, "not_found", "Owned resource not found"
	case errors.Is(err, persistence.ErrConflict):
		status, code, message = 409, "conflict", "Idempotency key or resource state conflicts"
	default:
		var e *provider.Error
		if errors.As(err, &e) {
			status, code, message = 422, string(e.Category), e.Message
			if e.Category == provider.Quota || e.Category == provider.RateLimited {
				status = 429
			}
		}
	}
	writeJSON(w, status, apiError(code, message, w.Header().Get("X-Request-ID")))
}
