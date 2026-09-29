package adapters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"
)

// Training is intentionally procedural, not an AI model. It exercises the real
// durable pipeline without keys, accounts, card details or hosted inference.
type Training struct{}

func (Training) ID() string { return "training" }
func (Training) DiscoverModels(context.Context, string) ([]provider.Model, error) {
	return []provider.Model{{ID: "training-landscape-v1", ProviderID: "training", DisplayName: "Training canvas", Capabilities: []provider.Capability{provider.ImageGeneration}, Selectable: true, Reason: "Procedural image for system-design exercises. Free, local, not AI."}}, nil
}
func (Training) TestConnection(context.Context, string) (provider.Health, error) {
	return provider.Health{Available: true, CheckedAt: time.Now().UTC()}, nil
}
func (Training) Submit(ctx context.Context, s provider.Submission) (provider.Operation, error) {
	w, h := 768, 432
	if s.AspectRatio == "1:1" {
		h = w
	}
	if s.AspectRatio == "9:16" {
		w, h = 432, 768
	}
	hash := sha256.Sum256([]byte(s.Prompt))
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		if ctx.Err() != nil {
			return provider.Operation{}, ctx.Err()
		}
		for x := 0; x < w; x++ {
			fy := float64(y) / float64(h)
			fx := float64(x) / float64(w)
			c := color.RGBA{uint8(20 + int(hash[0])%30 + int(fy*85)), uint8(35 + int(fy*60)), uint8(62 + int(fy*20)), 255}
			sunX := .25 + float64(hash[2]%40)/100
			sunY := .24
			d := math.Hypot((fx-sunX)*float64(w), (fy-sunY)*float64(h))
			if d < float64(h)*.085 {
				c = color.RGBA{244, 218, 172, 255}
			}
			for layer := 0; layer < 4; layer++ {
				ridge := .5 + float64(layer)*.105 + .08*math.Sin(fx*7+float64(hash[layer+3])) + .05*math.Sin(fx*18+float64(layer))
				if fy > ridge {
					shade := uint8(52 - layer*11)
					c = color.RGBA{shade, shade + 10, shade + 16, 255}
				}
			}
			im.SetRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		return provider.Operation{}, err
	}
	return provider.Operation{ID: s.GenerationID, Done: true, Artifacts: []provider.Artifact{{MIMEType: "image/png", Data: b.Bytes()}}}, nil
}
func (Training) Poll(context.Context, string, string) (provider.Operation, error) {
	return provider.Operation{Done: true}, nil
}
func (Training) Cancel(context.Context, string, string) error { return nil }
