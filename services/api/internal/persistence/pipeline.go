package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/events"
	"github.com/jackc/pgx/v5"
)

type StartFunc func(context.Context, Job) (string, error)
type PublishFunc func(context.Context, events.Envelope) error

func retryDelay(attempt int) int {
	if attempt > 6 {
		return 60
	}
	if attempt < 1 {
		return 1
	}
	return 1 << (attempt - 1)
}

// DispatchOne locks only one dispatch intent until its bounded Temporal call ends.
// A crash releases the lock; the stable workflow ID makes the next start safe.
func (s *Store) DispatchOne(ctx context.Context, start StartFunc) (bool, error) {
	return s.dispatchOne(ctx, nil, start)
}

// DispatchJob scopes a fault-injection probe to its own job in a shared dev DB.
func (s *Store) DispatchJob(ctx context.Context, jobID string, start StartFunc) (bool, error) {
	if !validID(jobID) {
		return false, ErrInvalid
	}
	return s.dispatchOne(ctx, jobID, start)
}

func (s *Store) dispatchOne(ctx context.Context, target any, start StartFunc) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	var job Job
	var attempt int
	err = tx.QueryRow(ctx, `SELECT id::text, owner_id::text, kind, workflow_id, dispatch_attempts + 1
		FROM jobs WHERE dispatch_state = 'pending' AND kind IN ('foundation_probe','image') AND dispatch_next_at <= now()
		AND ($1::uuid IS NULL OR id = $1::uuid)
		ORDER BY dispatch_next_at, created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED`, target).Scan(
		&job.ID, &job.OwnerID, &job.Kind, &job.WorkflowID, &attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	runID, startErr := start(ctx, job)
	if startErr == nil && runID == "" {
		startErr = errors.New("workflow start did not return a run ID")
	}
	if startErr != nil {
		_, err = tx.Exec(ctx, `UPDATE jobs SET dispatch_attempts = $2,
			dispatch_next_at = now() + make_interval(secs => $3) WHERE id = $1`, job.ID, attempt, float64(retryDelay(attempt)))
	} else {
		_, err = tx.Exec(ctx, `UPDATE jobs SET dispatch_state = 'started', workflow_run_id = $2,
            dispatch_attempts = $3, updated_at = now() WHERE id = $1`, job.ID, runID, attempt)
	}
	if err != nil {
		return true, err
	}
	if err = tx.Commit(ctx); err != nil {
		return true, err
	}
	if startErr != nil {
		return true, fmt.Errorf("workflow dispatch deferred: %w", startErr)
	}
	return true, nil
}

// RelayOne selects only the first unpublished event of an aggregate, even when
// that event is delayed or locked by another relay. A bounded publish happens
// while its row lock is held. No transaction spans a batch or provider call.
func (s *Store) RelayOne(ctx context.Context, publish PublishFunc) (bool, error) {
	return s.relayOne(ctx, nil, publish)
}

func (s *Store) RelayJob(ctx context.Context, jobID string, publish PublishFunc) (bool, error) {
	if !validID(jobID) {
		return false, ErrInvalid
	}
	return s.relayOne(ctx, jobID, publish)
}

func (s *Store) relayOne(ctx context.Context, target any, publish PublishFunc) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	var event events.Envelope
	event.SchemaVersion = 1
	var attempt int
	err = tx.QueryRow(ctx, `SELECT o.event_id::text, o.aggregate_id::text, o.sequence, o.event_type,
            o.occurred_at, o.publish_attempts + 1
		FROM outbox o WHERE o.published_at IS NULL AND o.next_attempt_at <= now()
		AND ($1::uuid IS NULL OR o.aggregate_id = $1::uuid)
        AND NOT EXISTS (SELECT 1 FROM outbox earlier WHERE earlier.aggregate_id = o.aggregate_id
            AND earlier.sequence < o.sequence AND earlier.published_at IS NULL)
		ORDER BY o.next_attempt_at, o.occurred_at, o.event_id LIMIT 1 FOR UPDATE OF o SKIP LOCKED`, target).Scan(
		&event.EventID, &event.AggregateID, &event.Sequence, &event.Type, &event.OccurredAt, &attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	publishErr := publish(ctx, event)
	if publishErr != nil {
		_, err = tx.Exec(ctx, `UPDATE outbox SET publish_attempts = $2,
			next_attempt_at = now() + make_interval(secs => $3) WHERE event_id = $1`, event.EventID, attempt, float64(retryDelay(attempt)))
	} else {
		_, err = tx.Exec(ctx, `UPDATE outbox SET published_at = now(), publish_attempts = $2 WHERE event_id = $1`, event.EventID, attempt)
	}
	if err != nil {
		return true, err
	}
	if err = tx.Commit(ctx); err != nil {
		return true, err
	}
	if publishErr != nil {
		return true, fmt.Errorf("event publish deferred: %w", publishErr)
	}
	return true, nil
}

func ValidateJobEvent(event events.Envelope) error {
	if event.Validate() != nil || !validID(event.EventID) || !validID(event.AggregateID) ||
		!validJobType(event.Type) {
		return ErrInvalid
	}
	return nil
}

func validJobType(value string) bool {
	switch value {
	case "job.queued", "job.submitting", "job.running", "job.succeeded", "job.failed", "job.reconciliation_required":
		return true
	}
	return false
}

// ApplyEvent commits the inbox identity and projection effect together. Older
// sequences are acknowledged without regressing state. The caller commits the
// Kafka offset only after this returns. IDs reused with altered content fail closed.
func (s *Store) ApplyEvent(ctx context.Context, consumer string, event events.Envelope) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if len(consumer) == 0 || len(consumer) > 120 || ValidateJobEvent(event) != nil {
		return false, ErrInvalid
	}
	// Canonical bytes make equivalent timestamp offsets hash identically.
	event.OccurredAt = event.OccurredAt.UTC()
	payload, err := json.Marshal(event)
	if err != nil {
		return false, err
	}
	hash := sha256.Sum256(payload)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	tag, err := tx.Exec(ctx, `INSERT INTO consumer_inbox(consumer_name, event_id, aggregate_id, sequence, payload_hash)
        VALUES ($1, $2, $3, $4, $5) ON CONFLICT (consumer_name, event_id) DO NOTHING`,
		consumer, event.EventID, event.AggregateID, event.Sequence, hash[:])
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		var same bool
		err = tx.QueryRow(ctx, `SELECT payload_hash = $3 FROM consumer_inbox WHERE consumer_name = $1 AND event_id = $2`,
			consumer, event.EventID, hash[:]).Scan(&same)
		if err != nil {
			return false, err
		}
		if !same {
			return false, ErrEventCollision
		}
		return false, tx.Commit(ctx)
	}
	tag, err = tx.Exec(ctx, `INSERT INTO job_event_projections(consumer_name, job_id, last_sequence, last_type)
        VALUES ($1, $2, $3, $4) ON CONFLICT (consumer_name, job_id) DO UPDATE
        SET last_sequence = EXCLUDED.last_sequence, last_type = EXCLUDED.last_type,
            applied_count = job_event_projections.applied_count + 1
        WHERE job_event_projections.last_sequence < EXCLUDED.last_sequence`, consumer, event.AggregateID, event.Sequence, event.Type)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, tx.Commit(ctx)
}
