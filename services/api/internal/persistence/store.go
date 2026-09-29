package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("owned job not found")
	ErrConflict       = errors.New("job identity conflicts with existing data")
	ErrInvalid        = errors.New("invalid pipeline input")
	ErrEventCollision = errors.New("event ID reused with different content")
)

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("OPEN_FLOW_DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL configuration")
	}
	cfg.MaxConns = 8
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("unable to initialize PostgreSQL pool")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("PostgreSQL connection failed")
	}
	return &Store{Pool: pool}, nil
}

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func validID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func WorkflowID(jobID string) string { return "open-flow.job." + jobID }

type Job struct {
	ID            string
	OwnerID       string
	Kind          string
	State         string
	WorkflowID    string
	DispatchState string
	Sequence      int64
}

// CreateProbeJob is an internal learning command, not a public generation API.
// The queued event and dispatch intent commit with the job.
func (s *Store) CreateProbeJob(ctx context.Context, ownerID, jobID string) error {
	if !validID(ownerID) || !validID(jobID) {
		return ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	tag, err := tx.Exec(ctx, `INSERT INTO jobs(id, owner_id, kind, workflow_id)
        VALUES ($1, $2, 'foundation_probe', $3) ON CONFLICT (id) DO NOTHING`, jobID, ownerID, WorkflowID(jobID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var matches bool
		err = tx.QueryRow(ctx, `SELECT owner_id = $2 AND kind = 'foundation_probe' FROM jobs WHERE id = $1`, jobID, ownerID).Scan(&matches)
		if err != nil {
			return err
		}
		if !matches {
			return ErrConflict
		}
	} else if _, err = tx.Exec(ctx, `INSERT INTO outbox(event_id, aggregate_id, sequence, event_type)
        VALUES ($1, $2, 1, 'job.queued')`, uuid.NewString(), jobID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Job(ctx context.Context, ownerID, jobID string) (Job, error) {
	if !validID(ownerID) || !validID(jobID) {
		return Job{}, ErrInvalid
	}
	var job Job
	err := s.Pool.QueryRow(ctx, `SELECT id::text, owner_id::text, kind, state, workflow_id, dispatch_state, sequence
        FROM jobs WHERE id = $1 AND owner_id = $2`, jobID, ownerID).Scan(
		&job.ID, &job.OwnerID, &job.Kind, &job.State, &job.WorkflowID, &job.DispatchState, &job.Sequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return job, err
}

// CompleteProbe is retry-safe even if an activity commits then loses its response.
func (s *Store) CompleteProbe(ctx context.Context, jobID string) error {
	if !validID(jobID) {
		return ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var state, kind string
	err = tx.QueryRow(ctx, `SELECT state, kind FROM jobs WHERE id = $1 FOR UPDATE`, jobID).Scan(&state, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if kind != "foundation_probe" {
		return ErrInvalid
	}
	if state == "succeeded" {
		return tx.Commit(ctx)
	}
	if state != "queued" {
		return ErrConflict
	}
	var sequence int64
	err = tx.QueryRow(ctx, `UPDATE jobs SET state = 'succeeded', sequence = sequence + 1, updated_at = now()
        WHERE id = $1 RETURNING sequence`, jobID).Scan(&sequence)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox(event_id, aggregate_id, sequence, event_type)
        VALUES ($1, $2, $3, 'job.succeeded')`, uuid.NewString(), jobID, sequence)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
