package adapters

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"net/http"
	"regexp"
	"time"
)

const FluxSchnell = "@cf/black-forest-labs/flux-1-schnell"

var accountIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

type CloudflareCredential struct {
	Token     string `json:"api_token"`
	AccountID string `json:"account_id"`
	FreePlan  bool   `json:"workers_free_confirmed"`
}
type Cloudflare struct {
	Resolve KeyResolver
	HTTP    *http.Client
	Base    string
}

func NewCloudflare(resolve KeyResolver) *Cloudflare {
	return &Cloudflare{Resolve: resolve, HTTP: boundedClient(), Base: "https://api.cloudflare.com/client/v4"}
}
func (*Cloudflare) ID() string { return "cloudflare" }
func (c *Cloudflare) credential(ctx context.Context, ref string) (CloudflareCredential, error) {
	raw, err := c.Resolve(ctx, ref)
	if err != nil {
		return CloudflareCredential{}, err
	}
	var key CloudflareCredential
	if json.Unmarshal([]byte(raw), &key) != nil || !accountIDPattern.MatchString(key.AccountID) || len(key.Token) < 10 || !key.FreePlan {
		return key, &provider.Error{Category: provider.Billing, Message: "Use a Workers Free account without a payment method", AcceptanceKnown: true}
	}
	return key, nil
}
func (c *Cloudflare) DiscoverModels(ctx context.Context, ref string) ([]provider.Model, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	key, err := c.credential(ctx, ref)
	if err != nil {
		return nil, err
	}
	data, err := request(ctx, c.HTTP, "GET", c.Base+"/accounts/"+key.AccountID+"/ai/models/search?search=flux-1-schnell&per_page=100", map[string]string{"Authorization": "Bearer " + key.Token}, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Success bool `json:"success"`
		Models  []struct {
			Name string `json:"name"`
		} `json:"result"`
	}
	if json.Unmarshal(data, &result) != nil || !result.Success {
		return nil, errors.New("Cloudflare model discovery failed")
	}
	models := []provider.Model{}
	for _, m := range result.Models {
		if m.Name == FluxSchnell {
			models = append(models, provider.Model{ID: m.Name, ProviderID: "cloudflare", DisplayName: "FLUX.1 Schnell", Capabilities: []provider.Capability{provider.ImageGeneration}, Selectable: true, Reason: "Workers Free: 10,000 neurons/day account-wide. Quota exhaustion stops requests; no upgrade or key cycling."})
		}
	}
	return models, nil
}
func (c *Cloudflare) TestConnection(ctx context.Context, ref string) (provider.Health, error) {
	start := time.Now()
	_, err := c.DiscoverModels(ctx, ref)
	return provider.Health{Available: err == nil, CheckedAt: time.Now().UTC(), Latency: time.Since(start)}, err
}
func (c *Cloudflare) Submit(ctx context.Context, s provider.Submission) (provider.Operation, error) {
	if s.ModelID != FluxSchnell {
		return provider.Operation{}, &provider.Error{Category: provider.Unsupported, Message: "Only the verified free-tier image model is enabled", AcceptanceKnown: true}
	}
	if s.AspectRatio != "1:1" {
		return provider.Operation{}, &provider.Error{Category: provider.Unsupported, Message: "This model supports square output in M1", AcceptanceKnown: true}
	}
	key, err := c.credential(ctx, s.CredentialRef)
	if err != nil {
		return provider.Operation{}, err
	}
	data, err := request(ctx, c.HTTP, "POST", c.Base+"/accounts/"+key.AccountID+"/ai/run/"+FluxSchnell, map[string]string{"Authorization": "Bearer " + key.Token}, map[string]any{"prompt": s.Prompt, "steps": 4})
	if err != nil {
		return provider.Operation{}, err
	}
	var response struct {
		Success bool `json:"success"`
		Result  struct {
			Image string `json:"image"`
		} `json:"result"`
	}
	if json.Unmarshal(data, &response) != nil || !response.Success {
		return provider.Operation{}, SafeError(errors.New("unusable generation response"), true)
	}
	body, err := base64.StdEncoding.DecodeString(response.Result.Image)
	if err != nil || len(body) > 16<<20 {
		return provider.Operation{}, SafeError(errors.New("invalid image"), true)
	}
	return provider.Operation{ID: s.GenerationID, Done: true, Artifacts: []provider.Artifact{{MIMEType: http.DetectContentType(body), Data: body}}}, nil
}
func (*Cloudflare) Poll(context.Context, string, string) (provider.Operation, error) {
	return provider.Operation{}, &provider.Error{Category: provider.Unsupported, Message: "Synchronous provider", AcceptanceKnown: true}
}
func (*Cloudflare) Cancel(context.Context, string, string) error {
	return &provider.Error{Category: provider.Unsupported, Message: "Cancellation unavailable", AcceptanceKnown: true}
}
