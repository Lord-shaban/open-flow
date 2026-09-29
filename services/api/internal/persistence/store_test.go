package persistence

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/events"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Each test creates its own schema; no existing application tables are touched.
func testStore(t *testing.T) *Store {
	t.Helper()
	databaseURL := os.Getenv("OPEN_FLOW_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration requires OPEN_FLOW_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	root, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(root.Pool.Close)
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = root.Pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := root.Pool.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store := &Store{Pool: pool}
	if err = store.Migrate(ctx, false); err != nil {
		t.Fatal(err)
	}
	return store
}

func probeJob(t *testing.T, s *Store) (string, string) {
	t.Helper()
	owner, job := uuid.NewString(), uuid.NewString()
	if _, err := s.Pool.Exec(context.Background(), `INSERT INTO owners(id, name) VALUES ($1, 'test owner')`, owner); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProbeJob(context.Background(), owner, job); err != nil {
		t.Fatal(err)
	}
	return owner, job
}

func TestMigrationRoundTripAndChecksum(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx, false); err != nil {
		t.Fatal(err)
	}
	var checksum string
	if err := s.Pool.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE version = 1`).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE schema_migrations SET checksum = 'tampered'`); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx, false); err == nil {
		t.Fatal("accepted altered applied migration")
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE schema_migrations SET checksum = $1`, checksum); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx, false); err != nil {
		t.Fatal(err)
	}
	var tables int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()`).Scan(&tables); err != nil || tables != 10 {
		t.Fatalf("tables=%d err=%v", tables, err)
	}
}

func TestJobOwnershipAndAtomicCompletion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner, job := probeJob(t, s)
	if err := s.CreateProbeJob(ctx, owner, job); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Job(ctx, uuid.NewString(), job); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.CreateProbeJob(ctx, uuid.NewString(), job); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if err := s.CompleteProbe(ctx, job); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	result, err := s.Job(ctx, owner, job)
	if err != nil || result.State != "succeeded" || result.Sequence != 2 {
		t.Fatalf("job=%+v err=%v", result, err)
	}
	var count int
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE aggregate_id = $1`, job).Scan(&count); err != nil || count != 2 {
		t.Fatalf("outbox=%d err=%v", count, err)
	}
	// Missing owner rejects the entire acceptance transaction, including its event.
	missingJob := uuid.NewString()
	if err = s.CreateProbeJob(ctx, uuid.NewString(), missingJob); err == nil {
		t.Fatal("missing owner accepted")
	}
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE id = $1`, missingJob).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial job commit")
	}
	otherOwner, credential := uuid.NewString(), uuid.NewString()
	if _, err = s.Pool.Exec(ctx, `INSERT INTO owners(id, name) VALUES ($1, 'other')`, otherOwner); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, `INSERT INTO credentials(id, owner_id, provider, name, encrypted_payload, key_version)
        VALUES ($1, $2, 'gemini', 'test', $3, 1)`, credential, otherOwner, make([]byte, 28)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, `UPDATE jobs SET credential_id = $2 WHERE id = $1`, job, credential); err == nil {
		t.Fatal("cross-owner credential accepted")
	}
}

func TestDispatcherLostDatabaseAcknowledgement(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner, jobID := probeJob(t, s)
	canceled, cancel := context.WithCancel(ctx)
	var firstID string
	_, err := s.DispatchOne(canceled, func(_ context.Context, job Job) (string, error) {
		firstID = job.WorkflowID
		cancel() // External start succeeded; process dies before SQL acknowledgement.
		return "existing-run", nil
	})
	if err == nil {
		t.Fatal("expected lost acknowledgement")
	}
	ok, err := s.DispatchOne(ctx, func(_ context.Context, job Job) (string, error) {
		if job.WorkflowID != firstID {
			t.Fatal("workflow identity changed")
		}
		return "existing-run", nil
	})
	if err != nil || !ok {
		t.Fatalf("retry: %v %v", ok, err)
	}
	job, err := s.Job(ctx, owner, jobID)
	if err != nil || job.DispatchState != "started" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	ok, err = s.DispatchOne(ctx, func(context.Context, Job) (string, error) {
		t.Fatal("dispatched acknowledged job twice")
		return "", nil
	})
	if ok || err != nil {
		t.Fatal(ok, err)
	}
}

func TestDispatcherRetryState(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, job := probeJob(t, s)
	if _, err := s.DispatchOne(ctx, func(context.Context, Job) (string, error) { return "", errors.New("unavailable") }); err == nil {
		t.Fatal("expected retry")
	}
	var attempts int
	var future bool
	if err := s.Pool.QueryRow(ctx, `SELECT dispatch_attempts, dispatch_next_at > now() FROM jobs WHERE id = $1`, job).Scan(&attempts, &future); err != nil || attempts != 1 || !future {
		t.Fatal(attempts, future, err)
	}
	if ok, err := s.DispatchOne(ctx, func(context.Context, Job) (string, error) { t.Fatal("ignored retry time"); return "", nil }); ok || err != nil {
		t.Fatal(ok, err)
	}
}

func TestRelayOrderingConcurrencyAndCrash(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, job := probeJob(t, s)
	if err := s.CompleteProbe(ctx, job); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	var first events.Envelope
	if _, err := s.RelayOne(canceled, func(_ context.Context, event events.Envelope) error { first = event; cancel(); return nil }); err == nil {
		t.Fatal("expected publish before crash")
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := s.RelayOne(ctx, func(_ context.Context, event events.Envelope) error {
			if event.EventID != first.EventID || event.Sequence != 1 {
				return errors.New("different event after crash")
			}
			close(entered)
			<-release
			return nil
		})
		done <- err
	}()
	<-entered
	ok, err := s.RelayOne(ctx, func(context.Context, events.Envelope) error { return errors.New("overtook locked predecessor") })
	close(release)
	if err != nil || ok {
		t.Fatalf("concurrent relay overtook predecessor: %v %v", ok, err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if ok, err = s.RelayOne(ctx, func(_ context.Context, event events.Envelope) error {
		if event.Sequence != 2 {
			t.Fatal("wrong next sequence")
		}
		return nil
	}); err != nil || !ok {
		t.Fatal(ok, err)
	}
}

func TestRelayRetryBlocksNewerEvent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, job := probeJob(t, s)
	if err := s.CompleteProbe(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RelayOne(ctx, func(context.Context, events.Envelope) error { return errors.New("Kafka down") }); err == nil {
		t.Fatal("expected Kafka failure")
	}
	if ok, err := s.RelayOne(ctx, func(context.Context, events.Envelope) error { t.Fatal("overtook delayed event"); return nil }); ok || err != nil {
		t.Fatal(ok, err)
	}
	var attempts int
	if err := s.Pool.QueryRow(ctx, `SELECT publish_attempts FROM outbox WHERE aggregate_id = $1 AND sequence = 1`, job).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatal(attempts, err)
	}
}

func TestInboxConcurrentDuplicatesAndStaleEvents(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, job := probeJob(t, s)
	event := events.Envelope{SchemaVersion: 1, EventID: uuid.NewString(), AggregateID: job, Sequence: 2, Type: "job.succeeded", OccurredAt: time.Now().UTC()}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if _, err := s.ApplyEvent(ctx, "test", event); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	stale := event
	stale.EventID = uuid.NewString()
	stale.Sequence = 1
	stale.Type = "job.queued"
	if applied, err := s.ApplyEvent(ctx, "test", stale); err != nil || applied {
		t.Fatal(applied, err)
	}
	altered := event
	altered.Type = "job.queued"
	if _, err := s.ApplyEvent(ctx, "test", altered); !errors.Is(err, ErrEventCollision) {
		t.Fatal(err)
	}
	var count, sequence int64
	if err := s.Pool.QueryRow(ctx, `SELECT applied_count, last_sequence FROM job_event_projections WHERE consumer_name = 'test' AND job_id = $1`, job).Scan(&count, &sequence); err != nil || count != 1 || sequence != 2 {
		t.Fatal(count, sequence, err)
	}
	// Independent consumers each receive their own durable effect.
	if applied, err := s.ApplyEvent(ctx, "another", event); err != nil || !applied {
		t.Fatal(applied, err)
	}
}

func TestRetryDelayBound(t *testing.T) {
	if retryDelay(1) != 1 || retryDelay(5) != 16 || retryDelay(100) != 60 {
		t.Fatal("unbounded retry")
	}
}

func TestOpenRedactsInvalidDSN(t *testing.T) {
	_, err := Open(context.Background(), "postgres://user:do-not-print@%broken")
	if err == nil || strings.Contains(err.Error(), "do-not-print") {
		t.Fatal(err)
	}
}
