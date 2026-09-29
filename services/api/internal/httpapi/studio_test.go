package httpapi

import (
	"bytes"
	"github.com/Lord-shaban/open-flow/services/api/internal/studio"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStudioRequiresOwnerAndRedactsLogs(t *testing.T) {
	var logs bytes.Buffer
	s := &studio.Service{OwnerToken: strings.Repeat("t", 32)}
	h := NewStudio(slog.New(slog.NewJSONHandler(&logs, nil)), s)
	for _, route := range []string{"/v1/credentials", "/v1/generations", "/v1/models?api_key=private-secret", "/v1/artifacts/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/download"} {
		req := httptest.NewRequest("GET", route, nil)
		req.Header.Set("Authorization", "Bearer private-secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
	if strings.Contains(logs.String(), "private-secret") {
		t.Fatal("secret in logs")
	}
}
func TestStudioJSONSizeAndUnknownFields(t *testing.T) {
	for _, body := range []string{`{"unknown":"private-key"}`, `{} {}`, strings.Repeat("a", 17000)} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		var in studio.CredentialInput
		if decode(w, r, &in) || w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), "private-key") {
			t.Fatal(w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", io.NopCloser(strings.NewReader(`{}`)))
	var in any
	if decode(w, r, &in) || w.Code != 415 {
		t.Fatal("content type accepted")
	}
}
func TestDownloadSignaturesAreOwnerAndExpiryBound(t *testing.T) {
	a := downloadSignature("key", "owner", "artifact", "123")
	if a == downloadSignature("key", "other", "artifact", "123") || a == downloadSignature("key", "owner", "artifact", "124") || a == downloadSignature("other-key", "owner", "artifact", "123") {
		t.Fatal("signature not bound")
	}
}

func TestExpiredAndOverlongSignedLinksDeniedBeforeStorage(t *testing.T) {
	s := &studio.Service{OwnerToken: strings.Repeat("t", 32), OwnerID: "owner"}
	handler := NewStudio(slog.New(slog.NewTextHandler(io.Discard, nil)), s)
	for _, stamp := range []time.Time{time.Now().Add(-time.Minute), time.Now().Add(3 * time.Minute)} {
		expires := strconv.FormatInt(stamp.Unix(), 10)
		path := "/v1/artifacts/00000000-0000-4000-8000-000000000003/content?expires=" + expires + "&signature=" + downloadSignature(s.OwnerToken, s.OwnerID, "00000000-0000-4000-8000-000000000003", expires)
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+s.OwnerToken)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		if response.Code != 404 {
			t.Fatal("expired/overlong signed URL accepted", response.Code)
		}
	}
}
