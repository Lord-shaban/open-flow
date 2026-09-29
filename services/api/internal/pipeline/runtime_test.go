package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"encoding/json"
	"github.com/Lord-shaban/open-flow/services/api/internal/events"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/segmentio/kafka-go"
)

type fakeReader struct {
	message   kafka.Message
	committed bool
	commitErr error
}

func (r *fakeReader) FetchMessage(context.Context) (kafka.Message, error) { return r.message, nil }
func (r *fakeReader) CommitMessages(context.Context, ...kafka.Message) error {
	r.committed = true
	return r.commitErr
}

type fakeStore struct {
	err     error
	applied bool
	called  bool
}

func (s *fakeStore) ApplyEvent(context.Context, string, events.Envelope) (bool, error) {
	s.called = true
	return s.applied, s.err
}

func TestConsumeCommitsAfterDurableEffects(t *testing.T) {
	event := events.Envelope{SchemaVersion: 1, EventID: "event", AggregateID: "job", Sequence: 1, Type: "job.queued", OccurredAt: time.Now()}
	value, _ := json.Marshal(event)
	for _, tc := range []struct {
		name             string
		dbErr, commitErr error
		wantCommit       bool
	}{
		{name: "durable duplicate is acknowledged", wantCommit: true},
		{name: "database failure preserves offset", dbErr: errors.New("unavailable")},
		{name: "offset failure can replay inbox", commitErr: errors.New("lost ack"), wantCommit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakeReader{message: kafka.Message{Key: []byte("job"), Value: value}, commitErr: tc.commitErr}
			store := &fakeStore{err: tc.dbErr}
			_, _, err := ConsumeOne(context.Background(), reader, store, "consumer")
			if (err != nil) != (tc.dbErr != nil || tc.commitErr != nil) || reader.committed != tc.wantCommit || !store.called {
				t.Fatalf("err=%v committed=%v called=%v", err, reader.committed, store.called)
			}
		})
	}
}

func TestConsumeRejectsPoisonAndWrongKeys(t *testing.T) {
	for _, value := range []string{`{`, `{"aggregate_id":"different"}`} {
		reader := &fakeReader{message: kafka.Message{Key: []byte("job"), Value: []byte(value)}}
		store := &fakeStore{}
		if _, _, err := ConsumeOne(context.Background(), reader, store, "consumer"); err == nil || reader.committed || store.called {
			t.Fatalf("poison message was acknowledged: %v", err)
		}
	}
}

func TestStartRejectsNonProbeWithoutCallingTemporal(t *testing.T) {
	s := TemporalStarter{}
	if _, err := s.Start(context.Background(), persistence.Job{Kind: "image"}); !errors.Is(err, persistence.ErrInvalid) {
		t.Fatal(err)
	}
}
