package workflows

import (
	"context"
	"errors"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const PersistedProbeWorkflowName = "open-flow.persisted-probe.v1"
const CompleteProbeActivityName = "open-flow.complete-probe.v1"

// PersistedProbe stores only a job ID in history. Provider workflows are OF-008.
func PersistedProbe(ctx workflow.Context, jobID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    5 * time.Second,
		ScheduleToCloseTimeout: time.Minute,
		RetryPolicy:            &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 5},
	})
	return workflow.ExecuteActivity(ctx, CompleteProbeActivityName, jobID).Get(ctx, nil)
}

type ProbeActivities struct{ Store *persistence.Store }

func (a *ProbeActivities) Complete(ctx context.Context, jobID string) error {
	err := a.Store.CompleteProbe(ctx, jobID)
	if errors.Is(err, persistence.ErrNotFound) || errors.Is(err, persistence.ErrInvalid) || errors.Is(err, persistence.ErrConflict) {
		return temporal.NewNonRetryableApplicationError("persisted probe rejected", "invalid_job", nil)
	}
	if err != nil {
		return errors.New("persisted probe database operation failed")
	}
	return nil
}
