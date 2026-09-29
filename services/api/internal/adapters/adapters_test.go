package adapters

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"github.com/google/uuid"
	"google.golang.org/genai"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGeminiVerifiedPaginationCacheAndNoBlindRetries(t *testing.T) {
	var calls atomic.Int32
	var submits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "secret-test-key" {
			t.Error("missing key header")
		}
		if strings.Contains(r.URL.Path, ":generateContent") {
			submits.Add(1)
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":{"code":503,"message":"secret-test-key prompt-private","status":"UNAVAILABLE"}}`))
			return
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "next" {
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-3.1-flash-image","supportedGenerationMethods":["generateContent"]}]}`))
		} else {
			_, _ = w.Write([]byte(`{"nextPageToken":"next","models":[{"name":"models/gemini-text-only","supportedGenerationMethods":["generateContent"]},{"name":"models/unknown-image-model","supportedGenerationMethods":["generateContent"]}]}`))
		}
	}))
	defer server.Close()
	g := NewGemini(func(context.Context, string) (string, error) { return "secret-test-key", nil })
	g.Factory = func(ctx context.Context, key string) (*genai.Client, error) {
		return genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI, HTTPClient: server.Client(), HTTPOptions: genai.HTTPOptions{BaseURL: server.URL, RetryOptions: &genai.HTTPRetryOptions{Attempts: genai.Ptr(int32(1))}}})
	}
	for range 2 {
		models, err := g.DiscoverModels(context.Background(), "ref")
		if err != nil || len(models) != 1 || models[0].ID != "gemini-3.1-flash-image" || models[0].Selectable {
			t.Fatalf("models=%+v err=%v", models, err)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("cache or pagination failed")
	}
	_, err := g.Submit(context.Background(), provider.Submission{ModelID: "gemini-3.1-flash-image", Prompt: "prompt-private", AspectRatio: "1:1"})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Category != provider.AmbiguousSubmission || strings.Contains(err.Error(), "secret-test-key") || strings.Contains(err.Error(), "prompt-private") || submits.Load() != 1 {
		t.Fatalf("unsafe submission calls=%d err=%v", submits.Load(), err)
	}
}
func TestHordeAsyncInlineImagesAndURLRejection(t *testing.T) {
	id := uuid.NewString()
	var payload map[string]any
	image, _ := Training{}.Submit(context.Background(), provider.Submission{Prompt: "fixture", AspectRatio: "1:1"})
	encoded := base64.StdEncoding.EncodeToString(image.Artifacts[0].Data)
	var remote atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/status/models":
			_, _ = w.Write([]byte(`[{"name":"Deliberate","count":2,"type":"image"},{"name":"Text","count":2,"type":"text"}]`))
		case r.Method == "POST":
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
		case strings.Contains(r.URL.Path, "/check/"):
			_ = json.NewEncoder(w).Encode(map[string]bool{"done": true})
		default:
			img := encoded
			if remote.Load() {
				img = "http://127.0.0.1/private"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"generations": []map[string]string{{"img": img}}})
		}
	}))
	defer server.Close()
	h := NewHorde(nil)
	h.Base = server.URL
	h.HTTP = server.Client()
	models, err := h.DiscoverModels(context.Background(), "")
	if err != nil || len(models) != 1 {
		t.Fatal(err)
	}
	op, err := h.Submit(context.Background(), provider.Submission{ModelID: "Deliberate", Prompt: "landscape", AspectRatio: "1:1"})
	if err != nil || op.ID != id || op.Done {
		t.Fatal(err)
	}
	if payload["r2"] != false || payload["shared"] != false {
		t.Fatal("unsafe result transport")
	}
	op, err = h.Poll(context.Background(), id, "")
	if err != nil || !op.Done || !bytes.Equal(op.Artifacts[0].Data, image.Artifacts[0].Data) {
		t.Fatal(err)
	}
	remote.Store(true)
	if _, err = h.Poll(context.Background(), id, ""); err == nil {
		t.Fatal("remote URL accepted")
	}
}
func TestCloudflareQuotaAndFreePlanGate(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(429)
		_, _ = w.Write([]byte("raw provider token secret"))
	}))
	defer server.Close()
	c := NewCloudflare(func(context.Context, string) (string, error) {
		return `{"api_token":"fixture-token","account_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","workers_free_confirmed":false}`, nil
	})
	c.Base = server.URL
	c.HTTP = server.Client()
	_, err := c.Submit(context.Background(), provider.Submission{ModelID: FluxSchnell, AspectRatio: "1:1"})
	if err == nil || calls.Load() != 0 {
		t.Fatal("unconfirmed plan reached provider")
	}
	c.Resolve = func(context.Context, string) (string, error) {
		return `{"api_token":"fixture-token","account_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","workers_free_confirmed":true}`, nil
	}
	_, err = c.Submit(context.Background(), provider.Submission{ModelID: FluxSchnell, AspectRatio: "1:1"})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Category != provider.Quota || !pe.AcceptanceKnown || calls.Load() != 1 || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
}
func TestProviderAuthenticationErrors(t *testing.T) {
	for _, code := range []int{401, 402, 403, 429, 503} {
		err := SafeError(genai.APIError{Code: code, Message: "do-not-leak"}, true)
		if strings.Contains(err.Error(), "do-not-leak") {
			t.Fatal(err)
		}
	}
}
