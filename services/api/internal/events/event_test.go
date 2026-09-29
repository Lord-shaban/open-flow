package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

type fakeWriter struct {
	messages []kafka.Message
	err      error
}

func (w *fakeWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.messages = append(w.messages, messages...)
	return w.err
}
func TestEventKeyAndContract(t *testing.T) {
	event := Envelope{SchemaVersion: 1, EventID: "evt-1", AggregateID: "job-1", Sequence: 1, Type: "foundation.probed", OccurredAt: time.Now().UTC()}
	w := &fakeWriter{}
	if err := Publish(context.Background(), w, event); err != nil {
		t.Fatal(err)
	}
	if string(w.messages[0].Key) != "job-1" {
		t.Fatal("partition key must be aggregate ID")
	}
	var decoded Envelope
	if err := json.Unmarshal(w.messages[0].Value, &decoded); err != nil || decoded.EventID != event.EventID {
		t.Fatal("invalid envelope encoding")
	}
	w.err = errors.New("broker unavailable")
	if err := Publish(context.Background(), w, event); !errors.Is(err, w.err) {
		t.Fatal("broker errors must propagate")
	}
}
func TestInvalidEventIsNeverPublished(t *testing.T) {
	w := &fakeWriter{}
	if err := Publish(context.Background(), w, Envelope{}); err == nil || len(w.messages) != 0 {
		t.Fatal("invalid events must not reach Kafka")
	}
}
