package adapters

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"google.golang.org/genai"
)

type KeyResolver func(context.Context, string) (string, error)
type ClientFactory func(context.Context, string) (*genai.Client, error)
type cachedModels struct {
	models  []provider.Model
	expires time.Time
}
type Gemini struct {
	Resolve KeyResolver
	Factory ClientFactory
	mu      sync.Mutex
	cache   map[[32]byte]cachedModels
}

func NewGemini(resolve KeyResolver) *Gemini {
	return &Gemini{Resolve: resolve, Factory: func(ctx context.Context, key string) (*genai.Client, error) {
		return genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI, HTTPClient: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, HTTPOptions: genai.HTTPOptions{RetryOptions: &genai.HTTPRetryOptions{Attempts: genai.Ptr(int32(1))}}})
	}, cache: map[[32]byte]cachedModels{}}
}
func (*Gemini) ID() string { return "gemini" }

// Verified against Google's image-generation guide on 2026-09-29. Never infer
// image output from generateContent, model descriptions or name substrings.
var imageModels = map[string]string{"gemini-3.1-flash-image": "Nano Banana 2", "gemini-3.1-flash-lite-image": "Nano Banana 2 Lite", "gemini-3-pro-image": "Nano Banana Pro"}

func (g *Gemini) DiscoverModels(ctx context.Context, ref string) ([]provider.Model, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	key, err := g.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(key))
	g.mu.Lock()
	cached, ok := g.cache[hash]
	g.mu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return slices.Clone(cached.models), nil
	}
	client, err := g.Factory(ctx, key)
	if err != nil {
		return nil, SafeError(err, false)
	}
	models := []provider.Model{}
	token := ""
	seen := map[string]bool{}
	for pages := 0; pages < 20; pages++ {
		page, err := client.Models.List(ctx, &genai.ListModelsConfig{PageSize: 100, PageToken: token})
		if err != nil {
			return nil, SafeError(err, false)
		}
		for _, m := range page.Items {
			if m == nil {
				continue
			}
			id := strings.TrimPrefix(m.Name, "models/")
			name, verified := imageModels[id]
			if !verified || !slices.Contains(m.SupportedActions, "generateContent") {
				continue
			}
			models = append(models, provider.Model{ID: id, ProviderID: "gemini", DisplayName: name, Capabilities: []provider.Capability{provider.ImageGeneration}, Selectable: false, Reason: "Google image API requires paid billing. Disabled by the no-card policy."})
		}
		token = page.NextPageToken
		if token == "" {
			g.mu.Lock()
			if len(g.cache) >= 64 {
				clear(g.cache)
			}
			g.cache[hash] = cachedModels{models: models, expires: time.Now().Add(5 * time.Minute)}
			g.mu.Unlock()
			return slices.Clone(models), nil
		}
		if seen[token] {
			return nil, &provider.Error{Category: provider.Transient, Message: "Model discovery pagination failed", AcceptanceKnown: true}
		}
		seen[token] = true
	}
	return nil, &provider.Error{Category: provider.Transient, Message: "Model discovery page limit exceeded", AcceptanceKnown: true}
}
func (g *Gemini) TestConnection(ctx context.Context, ref string) (provider.Health, error) {
	start := time.Now()
	g.mu.Lock()
	clear(g.cache)
	g.mu.Unlock()
	_, err := g.DiscoverModels(ctx, ref)
	return provider.Health{Available: err == nil, CheckedAt: time.Now().UTC(), Latency: time.Since(start)}, err
}
func (g *Gemini) Submit(ctx context.Context, s provider.Submission) (provider.Operation, error) {
	// Runtime admission prevents reaching this paid method in no-card mode.
	if _, ok := imageModels[s.ModelID]; !ok {
		return provider.Operation{}, &provider.Error{Category: provider.Unsupported, Message: "Unverified image model", AcceptanceKnown: true}
	}
	key, err := g.Resolve(ctx, s.CredentialRef)
	if err != nil {
		return provider.Operation{}, err
	}
	client, err := g.Factory(ctx, key)
	if err != nil {
		return provider.Operation{}, SafeError(err, false)
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	res, err := client.Models.GenerateContent(ctx, s.ModelID, genai.Text(s.Prompt), &genai.GenerateContentConfig{ResponseModalities: []string{"IMAGE"}, ImageConfig: &genai.ImageConfig{AspectRatio: s.AspectRatio}, HTTPOptions: &genai.HTTPOptions{RetryOptions: &genai.HTTPRetryOptions{Attempts: genai.Ptr(int32(1))}}})
	if err != nil {
		return provider.Operation{}, SafeError(err, true)
	}
	op := provider.Operation{ID: s.GenerationID, Done: true}
	for _, candidate := range res.Candidates {
		if candidate == nil || candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part != nil && part.InlineData != nil {
				op.Artifacts = append(op.Artifacts, provider.Artifact{MIMEType: part.InlineData.MIMEType, Data: part.InlineData.Data})
			}
		}
	}
	if len(op.Artifacts) == 0 {
		return op, &provider.Error{Category: provider.PolicyBlocked, Message: "Provider returned no image", AcceptanceKnown: true}
	}
	return op, nil
}
func (*Gemini) Poll(context.Context, string, string) (provider.Operation, error) {
	return provider.Operation{}, &provider.Error{Category: provider.Unsupported, Message: "Synchronous image submission has no polling operation", AcceptanceKnown: true}
}
func (*Gemini) Cancel(context.Context, string, string) error {
	return &provider.Error{Category: provider.Unsupported, Message: "Cancellation is unavailable", AcceptanceKnown: true}
}
func SafeError(err error, submission bool) error {
	var e genai.APIError
	category := provider.Transient
	known := !submission
	if errors.As(err, &e) {
		switch e.Code {
		case 400:
			category = provider.Unsupported
			known = true
		case 401, 403:
			category = provider.Authentication
			known = true
		case 402:
			category = provider.Billing
			known = true
		case 429:
			category = provider.Quota
			known = true
		}
	}
	if !known {
		category = provider.AmbiguousSubmission
	}
	return &provider.Error{Category: category, Message: "Provider request could not be completed", AcceptanceKnown: known}
}
