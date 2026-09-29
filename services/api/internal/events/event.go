package events

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/segmentio/kafka-go"
)

const FoundationTopic = "open-flow.foundation.v1"

// Envelope is versioned and contains no credentials, prompts or signed URLs.
type Envelope struct {
	SchemaVersion int       `json:"schema_version"`
	EventID       string    `json:"event_id"`
	AggregateID   string    `json:"aggregate_id"`
	Sequence      int64     `json:"sequence"`
	Type          string    `json:"type"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func (e Envelope) Validate() error {
	if e.SchemaVersion != 1 || e.EventID == "" || e.AggregateID == "" || e.Type == "" || e.Sequence < 1 || e.OccurredAt.IsZero() {
		return errors.New("invalid event envelope")
	}
	return nil
}

type Writer interface {
	WriteMessages(context.Context, ...kafka.Message) error
}

func Publish(ctx context.Context, writer Writer, event Envelope) error {
	if err := event.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return writer.WriteMessages(ctx, kafka.Message{Key: []byte(event.AggregateID), Value: payload})
}
