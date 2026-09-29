package adapters

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"time"
)

type Horde struct {
	Resolve KeyResolver
	HTTP    *http.Client
	Base    string
}

func NewHorde(resolve KeyResolver) *Horde {
	return &Horde{Resolve: resolve, HTTP: boundedClient(), Base: "https://aihorde.net/api/v2"}
}
func (*Horde) ID() string { return "horde" }
func (h *Horde) headers(ctx context.Context, ref string) (map[string]string, error) {
	key := "0000000000"
	if ref != "" {
		var err error
		key, err = h.Resolve(ctx, ref)
		if err != nil {
			return nil, err
		}
	}
	return map[string]string{"apikey": key, "Client-Agent": "open-flow:0.2.0:https://github.com/Lord-shaban/open-flow"}, nil
}
func (h *Horde) DiscoverModels(ctx context.Context, ref string) ([]provider.Model, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	headers, err := h.headers(ctx, ref)
	if err != nil {
		return nil, err
	}
	data, err := request(ctx, h.HTTP, "GET", h.Base+"/status/models?type=image&min_count=1&model_state=known", headers, nil)
	if err != nil {
		return nil, err
	}
	var models []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
		Type  string `json:"type"`
	}
	if json.Unmarshal(data, &models) != nil {
		return nil, errors.New("model discovery unavailable")
	}
	result := []provider.Model{}
	for _, m := range models {
		if m.Count < 1 || m.Name == "" || m.Type != "image" {
			continue
		}
		result = append(result, provider.Model{ID: m.Name, ProviderID: "horde", DisplayName: m.Name, Capabilities: []provider.Capability{provider.ImageGeneration}, Selectable: true, Reason: "Free community workers; availability and queue times vary. Anonymous results may be shared."})
		if len(result) >= 100 {
			break
		}
	}
	return result, nil
}
func (h *Horde) TestConnection(ctx context.Context, ref string) (provider.Health, error) {
	start := time.Now()
	headers, err := h.headers(ctx, ref)
	if err == nil {
		_, err = request(ctx, h.HTTP, "GET", h.Base+"/find_user", headers, nil)
	}
	return provider.Health{Available: err == nil, CheckedAt: time.Now().UTC(), Latency: time.Since(start)}, err
}
func (h *Horde) Submit(ctx context.Context, s provider.Submission) (provider.Operation, error) {
	headers, err := h.headers(ctx, s.CredentialRef)
	if err != nil {
		return provider.Operation{}, err
	}
	w, height := 512, 512
	if s.AspectRatio == "16:9" {
		w, height = 1024, 576
	}
	if s.AspectRatio == "9:16" {
		w, height = 576, 1024
	}
	payload := map[string]any{"prompt": s.Prompt, "params": map[string]any{"width": w, "height": height, "steps": 20, "n": 1, "sampler_name": "k_euler", "cfg_scale": 7, "karras": true}, "models": []string{s.ModelID}, "nsfw": false, "censor_nsfw": true, "trusted_workers": true, "r2": false, "shared": false, "slow_workers": true, "replacement_filter": true, "allow_downgrade": true}
	data, err := request(ctx, h.HTTP, "POST", h.Base+"/generate/async", headers, payload)
	if err != nil {
		return provider.Operation{}, err
	}
	var result struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &result) != nil || uuid.Validate(result.ID) != nil {
		return provider.Operation{}, SafeError(errors.New("missing acceptance ID"), true)
	}
	return provider.Operation{ID: result.ID}, nil
}
func (h *Horde) Poll(ctx context.Context, op, ref string) (provider.Operation, error) {
	if uuid.Validate(op) != nil {
		return provider.Operation{}, errors.New("invalid operation ID")
	}
	headers, err := h.headers(ctx, ref)
	if err != nil {
		return provider.Operation{}, err
	}
	data, err := request(ctx, h.HTTP, "GET", h.Base+"/generate/check/"+url.PathEscape(op), headers, nil)
	if err != nil {
		return provider.Operation{}, err
	}
	var check struct {
		Done    bool `json:"done"`
		Faulted bool `json:"faulted"`
	}
	if json.Unmarshal(data, &check) != nil {
		return provider.Operation{}, errors.New("invalid operation status")
	}
	if check.Faulted {
		return provider.Operation{}, &provider.Error{Category: provider.Transient, Message: "Community generation failed", AcceptanceKnown: true}
	}
	if !check.Done {
		return provider.Operation{ID: op}, nil
	}
	data, err = request(ctx, h.HTTP, "GET", h.Base+"/generate/status/"+op, headers, nil)
	if err != nil {
		return provider.Operation{}, err
	}
	var result struct {
		Generations []struct {
			Image    string `json:"img"`
			Censored bool   `json:"censored"`
		} `json:"generations"`
	}
	if json.Unmarshal(data, &result) != nil || len(result.Generations) != 1 {
		return provider.Operation{}, errors.New("invalid generation result")
	}
	g := result.Generations[0]
	if g.Censored {
		return provider.Operation{}, &provider.Error{Category: provider.PolicyBlocked, Message: "Provider censored the result", AcceptanceKnown: true}
	}
	body, err := base64.StdEncoding.DecodeString(g.Image)
	if err != nil || len(body) > 16<<20 {
		return provider.Operation{}, errors.New("invalid inline image; remote URLs are rejected")
	}
	return provider.Operation{ID: op, Done: true, Artifacts: []provider.Artifact{{Data: body, MIMEType: http.DetectContentType(body)}}}, nil
}
func (h *Horde) Cancel(ctx context.Context, op, ref string) error {
	if uuid.Validate(op) != nil {
		return errors.New("invalid operation ID")
	}
	headers, err := h.headers(ctx, ref)
	if err != nil {
		return err
	}
	_, err = request(ctx, h.HTTP, "DELETE", h.Base+"/generate/status/"+op, headers, nil)
	return err
}
