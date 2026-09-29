package workflows

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func TestPersistedProbeRetriesOnlyDatabaseActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	a := &ProbeActivities{}
	env.RegisterActivityWithOptions(a.Complete, activity.RegisterOptions{Name: CompleteProbeActivityName})
	env.OnActivity(CompleteProbeActivityName, mock.Anything, "job-id").Return(errors.New("db down")).Once()
	env.OnActivity(CompleteProbeActivityName, mock.Anything, "job-id").Return(nil).Once()
	env.ExecuteWorkflow(PersistedProbe, "job-id")
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	env.AssertExpectations(t)
}

func TestPersistedProbeDoesNotRetryRejectedJob(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	a := &ProbeActivities{}
	env.RegisterActivityWithOptions(a.Complete, activity.RegisterOptions{Name: CompleteProbeActivityName})
	env.OnActivity(CompleteProbeActivityName, mock.Anything, "bad-id").Return(temporal.NewNonRetryableApplicationError("rejected", "invalid_job", nil)).Once()
	env.ExecuteWorkflow(PersistedProbe, "bad-id")
	if env.GetWorkflowError() == nil {
		t.Fatal("expected rejection")
	}
	env.AssertExpectations(t)
}
