package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"github.com/google/uuid"
)

// ComfyUI talks only to a server-configured local inference endpoint. Clients
// cannot supply endpoints, arbitrary workflows, remote URLs or filesystem paths.
type ComfyUI struct {
	Endpoint string
	HTTP     *http.Client
}

func NewComfyUI(endpoint string) (*ComfyUI, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return nil, errors.New("invalid local ComfyUI endpoint")
	}
	host := u.Hostname()
	if host != "127.0.0.1" && host != "localhost" && host != "comfyui" && host != "host.docker.internal" {
		return nil, errors.New("ComfyUI endpoint must be local")
	}
	return &ComfyUI{Endpoint: strings.TrimRight(endpoint, "/"), HTTP: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (*ComfyUI) ID() string { return "comfyui" }
func (c *ComfyUI) request(ctx context.Context, method, route string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Endpoint+route, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("local provider status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (16<<20)+1))
	if len(data) > 16<<20 {
		return nil, errors.New("local provider response too large")
	}
	return data, err
}
func (c *ComfyUI) DiscoverModels(ctx context.Context, _ string) ([]provider.Model, error) {
	data, err := c.request(ctx, "GET", "/object_info/CheckpointLoaderSimple", nil)
	if err != nil {
		return nil, SafeError(err, false)
	}
	var doc struct {
		Loader struct {
			Input struct {
				Required struct {
					Names []json.RawMessage `json:"ckpt_name"`
				} `json:"required"`
			} `json:"input"`
		} `json:"CheckpointLoaderSimple"`
	}
	if json.Unmarshal(data, &doc) != nil || len(doc.Loader.Input.Required.Names) == 0 {
		return []provider.Model{}, nil
	}
	var names []string
	if json.Unmarshal(doc.Loader.Input.Required.Names[0], &names) != nil {
		return nil, errors.New("invalid checkpoint discovery")
	}
	result := []provider.Model{}
	for _, name := range names {
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
			continue
		}
		result = append(result, provider.Model{ID: name, ProviderID: "comfyui", DisplayName: name, Capabilities: []provider.Capability{provider.ImageGeneration}, Selectable: true, Reason: "Local installed checkpoint. No hosted API or card required."})
	}
	return result, nil
}
func (c *ComfyUI) TestConnection(ctx context.Context, ref string) (provider.Health, error) {
	start := time.Now()
	_, err := c.DiscoverModels(ctx, ref)
	return provider.Health{Available: err == nil, CheckedAt: time.Now().UTC(), Latency: time.Since(start)}, err
}
func (c *ComfyUI) Submit(ctx context.Context, s provider.Submission) (provider.Operation, error) {
	w, h := 768, 432
	if s.AspectRatio == "1:1" {
		w, h = 512, 512
	}
	if s.AspectRatio == "9:16" {
		w, h = 432, 768
	}
	node := func(kind string, inputs map[string]any) map[string]any {
		return map[string]any{"class_type": kind, "inputs": inputs}
	}
	ref := func(id string, index int) []any { return []any{id, index} }
	graph := map[string]any{
		"1": node("CheckpointLoaderSimple", map[string]any{"ckpt_name": s.ModelID}),
		"2": node("CLIPTextEncode", map[string]any{"text": s.Prompt, "clip": ref("1", 1)}),
		"3": node("CLIPTextEncode", map[string]any{"text": "", "clip": ref("1", 1)}),
		"4": node("EmptyLatentImage", map[string]any{"width": w, "height": h, "batch_size": 1}),
		"5": node("KSampler", map[string]any{"model": ref("1", 0), "positive": ref("2", 0), "negative": ref("3", 0), "latent_image": ref("4", 0), "seed": int64(1), "steps": 20, "cfg": 7, "sampler_name": "euler", "scheduler": "normal", "denoise": 1}),
		"6": node("VAEDecode", map[string]any{"samples": ref("5", 0), "vae": ref("1", 2)}),
		"7": node("SaveImage", map[string]any{"images": ref("6", 0), "filename_prefix": "open-flow-" + s.GenerationID})}
	data, err := c.request(ctx, "POST", "/prompt", map[string]any{"prompt": graph, "client_id": s.GenerationID})
	if err != nil {
		return provider.Operation{}, SafeError(err, true)
	}
	var result struct {
		ID string `json:"prompt_id"`
	}
	if json.Unmarshal(data, &result) != nil || uuid.Validate(result.ID) != nil {
		return provider.Operation{}, SafeError(errors.New("invalid acceptance"), true)
	}
	return provider.Operation{ID: result.ID, Done: false}, nil
}
func (c *ComfyUI) Poll(ctx context.Context, op, _ string) (provider.Operation, error) {
	if uuid.Validate(op) != nil {
		return provider.Operation{}, errors.New("invalid local operation")
	}
	data, err := c.request(ctx, "GET", "/history/"+op, nil)
	if err != nil {
		return provider.Operation{}, SafeError(err, false)
	}
	var history map[string]struct {
		Status struct {
			Completed bool   `json:"completed"`
			Status    string `json:"status_str"`
		} `json:"status"`
		Outputs map[string]struct {
			Images []struct{ Filename, Subfolder, Type string } `json:"images"`
		} `json:"outputs"`
	}
	if json.Unmarshal(data, &history) != nil {
		return provider.Operation{}, errors.New("invalid local history")
	}
	entry, found := history[op]
	result := provider.Operation{ID: op, Done: found && entry.Status.Completed}
	if entry.Status.Status == "error" {
		return result, &provider.Error{Category: provider.Unsupported, Message: "Local inference failed", AcceptanceKnown: true}
	}
	if !result.Done {
		return result, nil
	}
	for _, out := range entry.Outputs {
		for _, im := range out.Images {
			if im.Filename == "" || path.Base(im.Filename) != im.Filename || strings.Contains(im.Filename, "\\") || strings.Contains(im.Subfolder, "..") || strings.HasPrefix(im.Subfolder, "/") || strings.Contains(im.Subfolder, "\\") || im.Type != "output" {
				return result, errors.New("unsafe local artifact reference")
			}
			query := url.Values{"filename": {im.Filename}, "subfolder": {im.Subfolder}, "type": {"output"}}
			body, err := c.request(ctx, "GET", "/view?"+query.Encode(), nil)
			if err != nil {
				return result, err
			}
			result.Artifacts = append(result.Artifacts, provider.Artifact{Data: body, MIMEType: http.DetectContentType(body)})
			if len(result.Artifacts) > 1 {
				return result, errors.New("too many local artifacts")
			}
		}
	}
	if len(result.Artifacts) == 0 {
		return result, errors.New("local inference returned no image")
	}
	return result, nil
}
func (*ComfyUI) Cancel(context.Context, string, string) error {
	return &provider.Error{Category: provider.Unsupported, Message: "Cancellation belongs to M2", AcceptanceKnown: true}
}
