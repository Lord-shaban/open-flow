// image-smoke runs the complete M1 path against real PostgreSQL, Temporal,
// Kafka and private S3, with only the hosted provider HTTP endpoints replaced.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Lord-shaban/open-flow/services/api/internal/adapters"
	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/events"
	"github.com/Lord-shaban/open-flow/services/api/internal/httpapi"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/pipeline"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"github.com/Lord-shaban/open-flow/services/api/internal/storage"
	"github.com/Lord-shaban/open-flow/services/api/internal/studio"
	"github.com/Lord-shaban/open-flow/services/api/internal/vault"
	"github.com/Lord-shaban/open-flow/services/api/internal/workflows"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "M1 image smoke failed:", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := persistence.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Pool.Close()
	if err = db.Migrate(ctx, false); err != nil {
		return err
	}
	owner := uuid.NewString()
	if err = db.EnsureOwner(ctx, owner); err != nil {
		return err
	}
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	v, _ := vault.Parse(`{"1":"`+key+`"}`, 1)
	objects, err := storage.New(storage.Config{Endpoint: cfg.S3Endpoint, Region: cfg.S3Region, Bucket: "open-flow-smoke-" + owner, AccessKey: env("OPEN_FLOW_S3_ACCESS_KEY", "open-flow-ci"), SecretKey: env("OPEN_FLOW_S3_SECRET_KEY", "local-storage-ci-only")})
	if err != nil {
		return err
	}
	if _, err = objects.Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(objects.Bucket)}); err != nil {
		return errors.New("smoke private bucket creation failed")
	}
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 20*time.Second)
		defer c()
		_, _ = objects.Sweep(cleanup, map[string]bool{}, time.Now().Add(time.Minute), true)
		_, _ = objects.Client.DeleteBucket(cleanup, &s3.DeleteBucketInput{Bucket: aws.String(objects.Bucket)})
	}()
	service, err := studio.New(db, v, objects, owner, strings.Repeat("fixture-owner-token-", 3), "")
	if err != nil {
		return err
	}
	fixture, err := adapters.Training{}.Submit(ctx, provider.Submission{Prompt: "smoke", AspectRatio: "1:1"})
	if err != nil {
		return err
	}
	png := fixture.Artifacts[0].Data
	operation := uuid.NewString()
	var submissions atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/models"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "fixture-image", "count": 1, "type": "image"}})
		case r.Method == "POST":
			submissions.Add(1)
			var data map[string]any
			_ = json.NewDecoder(r.Body).Decode(&data)
			prompt, _ := data["prompt"].(string)
			if strings.Contains(prompt, "quota") {
				w.WriteHeader(429)
				_, _ = w.Write([]byte("private-fixture-key"))
				return
			}
			if strings.Contains(prompt, "ambiguous") {
				conn, _, _ := w.(http.Hijacker).Hijack()
				_ = conn.Close()
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"id": operation})
		case strings.Contains(r.URL.Path, "/check/"):
			_ = json.NewEncoder(w).Encode(map[string]bool{"done": true})
		case strings.Contains(r.URL.Path, "/status/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"generations": []map[string]string{{"img": base64.StdEncoding.EncodeToString(png)}}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		}
	}))
	defer remote.Close()
	horde := adapters.NewHorde(service.Resolve)
	horde.Base = remote.URL
	horde.HTTP = remote.Client()
	service.Adapters["horde"] = horde
	var logs bytes.Buffer
	server := httptest.NewServer(httpapi.NewStudio(slog.New(slog.NewJSONHandler(&logs, nil)), service))
	defer server.Close()
	call := func(method, path, body, idem string, auth bool) (int, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idem)
		if auth {
			req.Header.Set("Authorization", "Bearer "+service.OwnerToken)
		}
		res, err := server.Client().Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer res.Body.Close()
		data, err := io.ReadAll(io.LimitReader(res.Body, 20<<20))
		return res.StatusCode, data, err
	}
	code, _, err := call("GET", "/v1/credentials", "", "", false)
	if err != nil || code != 401 {
		return fmt.Errorf("owner protection status=%d", code)
	}
	credentialBody := `{"name":"fixture","provider":"horde","api_key":"private-fixture-key"}`
	code, data, err := call("POST", "/v1/credentials", credentialBody, "", true)
	if err != nil || code != 201 || bytes.Contains(data, []byte("private-fixture-key")) {
		return fmt.Errorf("credential creation status=%d", code)
	}
	var credential persistence.Credential
	_ = json.Unmarshal(data, &credential)
	code, _, err = call("POST", "/v1/credentials/"+credential.ID+"/test", "{}", "", true)
	if err != nil || code != 200 {
		return fmt.Errorf("connection test status=%d", code)
	}
	temporal, err := client.Dial(client.Options{HostPort: cfg.TemporalAddress, Namespace: cfg.TemporalNamespace})
	if err != nil {
		return err
	}
	defer temporal.Close()
	queue := "open-flow-image-smoke-" + owner
	w := worker.New(temporal, queue, worker.Options{})
	activities := &workflows.ImageActivities{Service: service}
	w.RegisterWorkflowWithOptions(workflows.Image, workflow.RegisterOptions{Name: workflows.ImageWorkflowName})
	w.RegisterActivityWithOptions(activities.Submit, activity.RegisterOptions{Name: workflows.SubmitImageActivity})
	w.RegisterActivityWithOptions(activities.Poll, activity.RegisterOptions{Name: workflows.PollImageActivity})
	w.RegisterActivityWithOptions(activities.Reconcile, activity.RegisterOptions{Name: workflows.ReconcileImageActivity})
	if err = w.Start(); err != nil {
		return err
	}
	defer w.Stop()
	starter := pipeline.TemporalStarter{Client: temporal, TaskQueue: queue}
	var success persistence.Generation
	for _, scenario := range []struct{ prompt, state string }{{"success-private-prompt", "succeeded"}, {"quota-private-prompt", "failed"}, {"ambiguous-private-prompt", "reconciliation_required"}} {
		payload, _ := json.Marshal(studio.GenerationInput{Provider: "horde", Model: "fixture-image", Prompt: scenario.prompt, Aspect: "1:1"})
		idem := uuid.NewString()
		code, data, err = call("POST", "/v1/generations", string(payload), idem, true)
		if err != nil || code != 202 {
			return fmt.Errorf("image acceptance status=%d response=%s", code, data)
		}
		var generation persistence.Generation
		_ = json.Unmarshal(data, &generation)
		code, again, err := call("POST", "/v1/generations", string(payload), idem, true)
		if err != nil || code != 202 {
			return errors.New("idempotent acceptance failed")
		}
		var duplicate persistence.Generation
		_ = json.Unmarshal(again, &duplicate)
		if duplicate.ID != generation.ID {
			return errors.New("duplicate created a job")
		}
		altered := strings.ReplaceAll(string(payload), scenario.prompt, "conflicting-prompt")
		code, _, _ = call("POST", "/v1/generations", altered, idem, true)
		if code != 409 {
			return errors.New("conflicting idempotency was accepted")
		}
		if _, err = db.DispatchJob(ctx, generation.ID, starter.Start); err != nil {
			return err
		}
		if err = temporal.GetWorkflow(ctx, persistence.WorkflowID(generation.ID), "").Get(ctx, nil); err != nil {
			return errors.New("image workflow failed")
		}
		generation, err = db.Generation(ctx, owner, generation.ID)
		if err != nil || generation.State != scenario.state {
			return fmt.Errorf("outcome=%s expected=%s err=%v", generation.State, scenario.state, err)
		}
		if scenario.state == "succeeded" {
			success = generation
		}
		// Inspect durable history; neither credential nor plaintext prompt may appear.
		history := temporal.GetWorkflowHistory(ctx, persistence.WorkflowID(generation.ID), "", false, 0)
		for history.HasNext() {
			event, e := history.Next()
			if e != nil {
				return e
			}
			encoded, _ := json.Marshal(event)
			if containsSecret(encoded, scenario.prompt) || containsSecret(encoded, "private-fixture-key") {
				return errors.New("secret in workflow history")
			}
		}
	}
	if submissions.Load() != 3 {
		return fmt.Errorf("unsafe submit retry: %d", submissions.Load())
	}
	if len(success.Artifacts) != 1 {
		return errors.New("missing private image")
	}
	artifact := success.Artifacts[0]
	code, data, err = call("GET", "/v1/artifacts/"+artifact.ID+"/download", "", "", true)
	if err != nil || code != 200 {
		return errors.New("signed download unavailable")
	}
	var download struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(data, &download)
	code, image, err := call("GET", download.URL, "", "", true)
	if err != nil || code != 200 || sha256.Sum256(image) != sha256.Sum256(png) {
		return errors.New("image download mismatch")
	}
	code, _, _ = call("GET", download.URL, "", "", false)
	if code != 401 {
		return errors.New("download bypassed owner authorization")
	}
	code, _, _ = call("GET", strings.Replace(download.URL, "expires=", "expires=0", 1), "", "", true)
	if code != 404 {
		return errors.New("tampered expiry accepted")
	}
	stored, err := db.Artifact(ctx, owner, artifact.ID)
	if err != nil {
		return err
	}
	unsigned, err := http.Get(cfg.S3Endpoint + "/" + objects.Bucket + "/" + stored.Key)
	if err != nil {
		return err
	}
	_ = unsigned.Body.Close()
	if unsigned.StatusCode != 403 {
		return fmt.Errorf("bucket is not private: %d", unsigned.StatusCode)
	}
	writer := pipeline.NewWriter(cfg.KafkaBroker)
	defer writer.Close()
	publish := func(ctx context.Context, e events.Envelope) error {
		b, _ := json.Marshal(e)
		return writer.WriteMessages(ctx, kafka.Message{Key: []byte(e.AggregateID), Value: b})
	}
	for {
		work, err := db.RelayJob(ctx, success.ID, publish)
		if err != nil {
			return err
		}
		if !work {
			break
		}
	}
	reader := pipeline.NewReader(cfg.KafkaBroker, "open-flow-image-smoke-"+owner)
	defer reader.Close()
	seen := []string{}
	for len(seen) < 4 {
		e, _, err := pipeline.ConsumeOne(ctx, reader, db, "image-smoke-"+owner)
		if err != nil {
			return err
		}
		if e.AggregateID == success.ID {
			seen = append(seen, e.Type)
		}
	}
	if strings.Join(seen, ",") != "job.queued,job.submitting,job.running,job.succeeded" {
		return fmt.Errorf("invalid image lifecycle: %v", seen)
	}
	code, _, _ = call("DELETE", "/v1/generations/"+success.ID, "", "", true)
	if code != 200 {
		return errors.New("owned image deletion failed")
	}
	code, _, _ = call("GET", download.URL, "", "", true)
	if code != 404 {
		return errors.New("deleted download still available")
	}
	refs, err := db.ReferencedKeys(ctx)
	if err != nil {
		return err
	}
	count, err := objects.Sweep(ctx, refs, time.Now().Add(time.Minute), true)
	if err != nil || count != 1 {
		return errors.New("retention cleanup failed")
	}
	if strings.Contains(logs.String(), "private-fixture-key") || strings.Contains(logs.String(), "success-private-prompt") {
		return errors.New("secret in HTTP logs")
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "ok", "provider_http_fixtures": true, "real_infrastructure": []string{"PostgreSQL", "Temporal", "Kafka", "S3"}, "owner_protection": true, "encrypted_credentials": true, "idempotency": true, "image_download": true, "ambiguous_submissions_not_retried": true, "submissions": submissions.Load(), "events": seen, "private_bucket": true, "retention_cleanup": true})
	return nil
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Protobuf JSON base64-encodes payload bytes; inspect decoded values as well.
func containsSecret(encoded []byte, secret string) bool {
	if bytes.Contains(encoded, []byte(secret)) {
		return true
	}
	var node any
	if json.Unmarshal(encoded, &node) != nil {
		return false
	}
	var walk func(any) bool
	walk = func(value any) bool {
		switch x := value.(type) {
		case string:
			decoded, err := base64.StdEncoding.DecodeString(x)
			return err == nil && bytes.Contains(decoded, []byte(secret))
		case []any:
			for _, item := range x {
				if walk(item) {
					return true
				}
			}
		case map[string]any:
			for _, item := range x {
				if walk(item) {
					return true
				}
			}
		}
		return false
	}
	return walk(node)
}
