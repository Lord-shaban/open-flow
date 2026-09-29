package workflows

import (
	"errors"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"testing"
)

func TestImageNeverResubmitsAmbiguousActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	a := &ImageActivities{}
	env.RegisterActivityWithOptions(a.Submit, activity.RegisterOptions{Name: SubmitImageActivity})
	env.RegisterActivityWithOptions(a.Reconcile, activity.RegisterOptions{Name: ReconcileImageActivity})
	env.OnActivity(SubmitImageActivity, mock.Anything, "job-id").Return("", errors.New("response lost")).Once()
	env.OnActivity(ReconcileImageActivity, mock.Anything, "job-id", false).Return("reconciliation_required", nil).Once()
	env.ExecuteWorkflow(Image, "job-id")
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	env.AssertExpectations(t)
}
func TestImageResumesKnownOperationAndPolls(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	a := &ImageActivities{}
	env.RegisterActivityWithOptions(a.Submit, activity.RegisterOptions{Name: SubmitImageActivity})
	env.RegisterActivityWithOptions(a.Poll, activity.RegisterOptions{Name: PollImageActivity})
	env.OnActivity(SubmitImageActivity, mock.Anything, "job-id").Return("running", nil).Once()
	env.OnActivity(PollImageActivity, mock.Anything, "job-id").Return("running", nil).Once()
	env.OnActivity(PollImageActivity, mock.Anything, "job-id").Return("succeeded", nil).Once()
	env.ExecuteWorkflow(Image, "job-id")
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	env.AssertExpectations(t)
}
