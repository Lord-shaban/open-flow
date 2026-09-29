package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/events"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/pipeline"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	store, err := persistence.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Pool.Close()
	ownerID, jobID := uuid.NewString(), uuid.NewString()
	if _, err = store.Pool.Exec(ctx, `INSERT INTO owners(id, name) VALUES ($1, 'pipeline learning probe')`, ownerID); err != nil {
		return err
	}
	if err = store.CreateProbeJob(ctx, ownerID, jobID); err != nil {
		return err
	}
	connection, err := client.DialContext(ctx, client.Options{HostPort: cfg.TemporalAddress, Namespace: cfg.TemporalNamespace})
	if err != nil {
		return err
	}
	defer connection.Close()
	starter := pipeline.TemporalStarter{Client: connection, TaskQueue: cfg.TemporalTaskQueue}

	// Start was accepted but the dispatcher dies before its database commit.
	lostAck, loseAck := context.WithCancel(ctx)
	var originalRun string
	_, err = store.DispatchJob(lostAck, jobID, func(ctx context.Context, job persistence.Job) (string, error) {
		runID, startErr := starter.Start(ctx, job)
		if startErr == nil {
			originalRun = runID
			loseAck()
		}
		return runID, startErr
	})
	loseAck()
	if err == nil || originalRun == "" {
		return errors.New("dispatcher failure window was not exercised")
	}
	// Wait for this run to finish before retrying, proving completed-ID reuse is denied too.
	if err = connection.GetWorkflow(ctx, persistence.WorkflowID(jobID), originalRun).Get(ctx, nil); err != nil {
		return err
	}
	var recoveredRun string
	if _, err = store.DispatchJob(ctx, jobID, func(ctx context.Context, job persistence.Job) (string, error) {
		runID, startErr := starter.Start(ctx, job)
		recoveredRun = runID
		return runID, startErr
	}); err != nil {
		return fmt.Errorf("workflow start recovery failed: %w", err)
	}
	if recoveredRun != originalRun {
		return errors.New("dispatcher created a second workflow run")
	}
	job, err := store.Job(ctx, ownerID, jobID)
	if err != nil || job.State != "succeeded" || job.DispatchState != "started" || job.Sequence != 2 {
		return errors.New("persisted job did not complete")
	}

	writer := pipeline.NewWriter(cfg.KafkaBroker)
	defer writer.Close()
	// Kafka acknowledged the publish; the relay dies before marking its row.
	lostPublishAck, losePublishAck := context.WithCancel(ctx)
	var originalEvent events.Envelope
	_, err = store.RelayJob(lostPublishAck, jobID, func(ctx context.Context, event events.Envelope) error {
		if publishErr := events.Publish(ctx, writer, event); publishErr != nil {
			return publishErr
		}
		originalEvent = event
		losePublishAck()
		return nil
	})
	losePublishAck()
	if err == nil || originalEvent.EventID == "" {
		return errors.New("relay failure window was not exercised")
	}
	for range 2 {
		worked, relayErr := store.RelayJob(ctx, jobID, func(ctx context.Context, event events.Envelope) error { return events.Publish(ctx, writer, event) })
		if relayErr != nil || !worked {
			return errors.New("outbox retry failed")
		}
	}
	// Dedicated group replays this topic through explicit Fetch/Commit APIs.
	consumer := "probe-" + jobID
	reader := pipeline.NewReader(cfg.KafkaBroker, consumer)
	defer reader.Close()
	sequences := make([]int64, 0, 3)
	duplicateCount := 0
	for len(sequences) < 3 {
		event, applied, consumeErr := pipeline.ConsumeOne(ctx, reader, store, consumer)
		if consumeErr != nil {
			return consumeErr
		}
		if event.AggregateID != jobID {
			continue
		}
		sequences = append(sequences, event.Sequence)
		if !applied {
			duplicateCount++
		}
	}
	if sequences[0] != 1 || sequences[1] != 1 || sequences[2] != 2 || duplicateCount != 1 {
		return fmt.Errorf("unexpected event order/effects: %v duplicates=%d", sequences, duplicateCount)
	}
	var count, sequence int64
	if err = store.Pool.QueryRow(ctx, `SELECT applied_count, last_sequence FROM job_event_projections
        WHERE consumer_name = $1 AND job_id = $2`, consumer, jobID).Scan(&count, &sequence); err != nil || count != 2 || sequence != 2 {
		return errors.New("duplicate event changed durable projection")
	}
	var pending int
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND published_at IS NULL`, jobID).Scan(&pending); err != nil || pending != 0 {
		return errors.New("outbox was not acknowledged")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"job_id": jobID, "workflow_run_reused": recoveredRun == originalRun,
		"event_sequences": sequences, "duplicates_ignored": duplicateCount, "projection_effects": count, "status": "ok",
	})
}
