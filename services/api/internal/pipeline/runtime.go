package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/events"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/workflows"
	"github.com/segmentio/kafka-go"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

const JobTopic = "open-flow.jobs.v1"
const ProjectionConsumer = "open-flow-job-projection-v1"

type TemporalStarter struct {
	Client    client.Client
	TaskQueue string
}

func (s TemporalStarter) Start(ctx context.Context, job persistence.Job) (string, error) {
	if (job.Kind != "foundation_probe" && job.Kind != "image") || job.WorkflowID != persistence.WorkflowID(job.ID) {
		return "", persistence.ErrInvalid
	}
	name := workflows.PersistedProbeWorkflowName
	if job.Kind == "image" {
		name = workflows.ImageWorkflowName
	}
	run, err := s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: job.WorkflowID, TaskQueue: s.TaskQueue,
		WorkflowIDConflictPolicy:                 enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
		WorkflowIDReusePolicy:                    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
		WorkflowExecutionTimeout:                 2 * time.Hour,
	}, name, job.ID)
	var existing *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &existing) {
		return existing.RunId, nil
	}
	if err != nil {
		return "", err
	}
	return run.GetRunID(), nil
}

func NewWriter(broker string) *kafka.Writer {
	return &kafka.Writer{Addr: kafka.TCP(broker), Topic: JobTopic, Balancer: &kafka.Hash{},
		RequiredAcks: kafka.RequireAll, MaxAttempts: 3, WriteTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second,
		BatchTimeout: 10 * time.Millisecond}
}

func NewReader(broker, group string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{Brokers: []string{broker}, Topic: JobTopic, GroupID: group,
		StartOffset: kafka.FirstOffset, MinBytes: 1, MaxBytes: 1 << 20, MaxWait: time.Second, CommitInterval: 0})
}

type ConsumerStore interface {
	ApplyEvent(context.Context, string, events.Envelope) (bool, error)
}
type MessageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
}

func ConsumeOne(ctx context.Context, reader MessageReader, store ConsumerStore, consumer string) (events.Envelope, bool, error) {
	message, err := reader.FetchMessage(ctx)
	if err != nil {
		return events.Envelope{}, false, err
	}
	var event events.Envelope
	decoder := json.NewDecoder(bytes.NewReader(message.Value))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&event); err != nil {
		return event, false, errors.New("invalid job event JSON; offset not committed")
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return event, false, errors.New("trailing job event JSON; offset not committed")
	}
	if string(message.Key) != event.AggregateID {
		return event, false, errors.New("job event partition key mismatch; offset not committed")
	}
	applied, err := store.ApplyEvent(ctx, consumer, event)
	if err != nil {
		return event, false, err
	}
	if err = reader.CommitMessages(ctx, message); err != nil {
		return event, applied, err
	}
	return event, applied, nil
}

// RunLoop persists retry scheduling in PostgreSQL. Raw dependency errors are not
// logged because future drivers/providers may include sensitive response text.
func RunLoop(ctx context.Context, name string, step func(context.Context) (bool, error)) error {
	for ctx.Err() == nil {
		work, err := step(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Warn("pipeline step deferred", "process", name)
		}
		if err != nil || !work {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
		}
	}
	return nil
}

func RunConsumer(ctx context.Context, reader MessageReader, store ConsumerStore) error {
	for ctx.Err() == nil {
		if _, _, err := ConsumeOne(ctx, reader, store, ProjectionConsumer); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Stop on poison messages; operators fix/redrive before restarting.
			return errors.New("job consumer stopped before committing offset")
		}
	}
	return nil
}
