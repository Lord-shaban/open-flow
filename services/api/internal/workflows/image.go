package workflows

import (
	"bytes"
	"context"
	"errors"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"github.com/Lord-shaban/open-flow/services/api/internal/storage"
	"github.com/Lord-shaban/open-flow/services/api/internal/studio"
	"github.com/Lord-shaban/open-flow/services/api/internal/vault"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"time"
)

const ImageWorkflowName = "open-flow.image.v1"
const SubmitImageActivity = "open-flow.image.submit.v1"
const PollImageActivity = "open-flow.image.poll.v1"
const ReconcileImageActivity = "open-flow.image.reconcile.v1"

// Image stores only job IDs and coarse states in history. Submission has exactly
// one activity attempt; replay never retries an unknown external acceptance.
func Image(ctx workflow.Context, id string) error {
	submit := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 110 * time.Second, ScheduleToCloseTimeout: 2 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	reads := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 35 * time.Second, ScheduleToCloseTimeout: 2 * time.Minute, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3}})
	var state string
	if err := workflow.ExecuteActivity(submit, SubmitImageActivity, id).Get(ctx, &state); err != nil {
		if err = workflow.ExecuteActivity(reads, ReconcileImageActivity, id, false).Get(ctx, &state); err != nil {
			return err
		}
	}
	for poll := 0; state == "running" && poll < 360; poll++ {
		if err := workflow.Sleep(ctx, 5*time.Second); err != nil {
			return err
		}
		if err := workflow.ExecuteActivity(reads, PollImageActivity, id).Get(ctx, &state); err != nil {
			if err = workflow.ExecuteActivity(reads, ReconcileImageActivity, id, false).Get(ctx, &state); err != nil {
				return err
			}
			if state == "running" {
				continue
			}
		}
	}
	if state == "running" {
		_, err := finishDeferred(ctx, reads, id)
		return err
	}
	return nil
}
func finishDeferred(ctx, reads workflow.Context, id string) (string, error) {
	var state string
	err := workflow.ExecuteActivity(reads, ReconcileImageActivity, id, true).Get(ctx, &state)
	return state, err
}

type ImageActivities struct{ Service *studio.Service }

func (a *ImageActivities) Submit(ctx context.Context, id string) (string, error) {
	g, claimed, err := a.Service.Store.ClaimSubmission(ctx, id)
	if err != nil {
		return "", errors.New("image submission intent unavailable")
	}
	if !claimed {
		if g.State == "submitting" {
			return a.fail(ctx, g, "reconciliation_required", "ambiguous_submission")
		}
		return g.State, nil
	}
	if g.OwnerID != a.Service.OwnerID {
		return a.fail(ctx, g, "failed", "owner_mismatch")
	}
	if g.Provider == "gemini" {
		return a.fail(ctx, g, "failed", "billing_disabled")
	}
	adapter := a.Service.Adapters[g.Provider]
	if adapter == nil {
		return a.fail(ctx, g, "failed", "unsupported")
	}
	prompt, err := a.Service.Vault.Open(g.Ciphertext, g.Version, vault.AAD(g.OwnerID, g.Provider, g.ID, "prompt"))
	if err != nil {
		return a.fail(ctx, g, "failed", "prompt_decryption")
	}
	if g.CredentialID != "" {
		if _, err = a.Service.Resolve(ctx, g.CredentialID); err != nil {
			return a.fail(ctx, g, "failed", "authentication")
		}
	}
	op, err := adapter.Submit(ctx, provider.Submission{GenerationID: id, ModelID: g.Model, CredentialRef: g.CredentialID, Prompt: string(prompt), AspectRatio: g.AspectRatio, Capability: provider.ImageGeneration})
	if err != nil {
		var pe *provider.Error
		if errors.As(err, &pe) && pe.AcceptanceKnown {
			return a.fail(ctx, g, "failed", string(pe.Category))
		}
		return a.fail(ctx, g, "reconciliation_required", "ambiguous_submission")
	}
	if !op.Done {
		if err = a.Service.Store.AcceptOperation(ctx, id, op.ID); err != nil {
			return "", errors.New("operation acceptance persistence unavailable")
		}
		return "running", nil
	}
	return a.persist(ctx, g, op)
}
func (a *ImageActivities) Poll(ctx context.Context, id string) (string, error) {
	g, _, err := a.Service.Store.ClaimSubmission(ctx, id)
	if err != nil {
		return "", errors.New("operation state unavailable")
	}
	if g.State != "running" {
		return g.State, nil
	}
	adapter := a.Service.Adapters[g.Provider]
	if adapter == nil {
		return a.fail(ctx, g, "reconciliation_required", "provider_unavailable")
	}
	op, err := adapter.Poll(ctx, g.Operation, g.CredentialID)
	if err != nil {
		var pe *provider.Error
		if errors.As(err, &pe) && (pe.Category == provider.PolicyBlocked || pe.Category == provider.Unsupported) {
			return a.fail(ctx, g, "failed", string(pe.Category))
		}
		return "", errors.New("operation poll deferred")
	}
	if !op.Done {
		return "running", nil
	}
	return a.persist(ctx, g, op)
}

// Reconcile cannot resubmit. A known persisted operation can safely keep polling.
func (a *ImageActivities) Reconcile(ctx context.Context, id string, exhausted bool) (string, error) {
	g, _, err := a.Service.Store.ClaimSubmission(ctx, id)
	if err != nil {
		return "", errors.New("reconciliation state unavailable")
	}
	if g.State == "succeeded" || g.State == "failed" || g.State == "reconciliation_required" {
		return g.State, nil
	}
	if g.State == "running" && !exhausted {
		return "running", nil
	}
	return a.fail(ctx, g, "reconciliation_required", "ambiguous_submission")
}
func (a *ImageActivities) fail(ctx context.Context, g persistence.Generation, state, code string) (string, error) {
	if err := a.Service.Store.Finish(ctx, g.ID, state, code, nil); err != nil {
		return "", errors.New("image outcome persistence unavailable")
	}
	return state, nil
}
func (a *ImageActivities) persist(ctx context.Context, g persistence.Generation, op provider.Operation) (string, error) {
	if len(op.Artifacts) != 1 {
		return a.fail(ctx, g, "reconciliation_required", "invalid_provider_artifact")
	}
	artifacts := []persistence.Artifact{}
	for index, result := range op.Artifacts {
		size := int64(len(result.Data))
		mime := http.DetectContentType(result.Data)
		if size < 1 || size > storage.MaxImageBytes || mime != result.MIMEType || mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" {
			return a.fail(ctx, g, "reconciliation_required", "invalid_provider_artifact")
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(result.Data))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 32_000_000 {
			return a.fail(ctx, g, "reconciliation_required", "invalid_provider_artifact")
		}
		artifactID := uuid.NewSHA1(uuid.MustParse(g.ID), []byte{byte(index)}).String()
		key := "open-flow/" + g.OwnerID + "/" + g.ID + "/" + artifactID
		if err = a.Service.Storage.Put(ctx, key, mime, size, bytes.NewReader(result.Data)); err != nil {
			return "", errors.New("image upload deferred")
		}
		artifacts = append(artifacts, persistence.Artifact{ID: artifactID, JobID: g.ID, OwnerID: g.OwnerID, Key: key, ContentType: mime, Bytes: size, ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)})
	}
	if err := a.Service.Store.Finish(ctx, g.ID, "succeeded", "", artifacts); err != nil {
		return "", errors.New("image completion persistence unavailable")
	}
	return "succeeded", nil
}
