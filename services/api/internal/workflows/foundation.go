package workflows

import (
	"context"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const FoundationWorkflowName = "open-flow.foundation.v1"
const ProbeActivityName = "open-flow.probe.v1"

type ProbeInput struct{ CorrelationID string }
type ProbeResult struct {
	CorrelationID string
	Status        string
}

// Foundation is a deterministic learning probe; it does not submit paid provider calls.
func Foundation(ctx workflow.Context, input ProbeInput) (ProbeResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    5 * time.Second,
		ScheduleToCloseTimeout: 20 * time.Second,
		RetryPolicy:            &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3},
	})
	var result ProbeResult
	err := workflow.ExecuteActivity(ctx, ProbeActivityName, input).Get(ctx, &result)
	return result, err
}

func Probe(_ context.Context, input ProbeInput) (ProbeResult, error) {
	if input.CorrelationID == "" {
		return ProbeResult{}, temporal.NewNonRetryableApplicationError("correlation ID is required", "invalid_input", nil)
	}
	return ProbeResult{CorrelationID: input.CorrelationID, Status: "ok"}, nil
}
