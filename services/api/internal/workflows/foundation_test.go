package workflows

import (
	"context"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestFoundationWorkflow(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(Probe, activity.RegisterOptions{Name: ProbeActivityName})
	env.ExecuteWorkflow(Foundation, ProbeInput{CorrelationID: "test-job"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result ProbeResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.CorrelationID != "test-job" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestInvalidProbeIsNotRetried(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	calls := 0
	env.RegisterActivityWithOptions(func(ctx context.Context, input ProbeInput) (ProbeResult, error) {
		calls++
		return Probe(ctx, input)
	}, activity.RegisterOptions{Name: ProbeActivityName})
	env.ExecuteWorkflow(Foundation, ProbeInput{})
	if env.GetWorkflowError() == nil || calls != 1 {
		t.Fatalf("expected one permanent failure, calls=%d", calls)
	}
}
