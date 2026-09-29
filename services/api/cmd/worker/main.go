package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/studio"
	"github.com/Lord-shaban/open-flow/services/api/internal/workflows"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	connection, err := client.Dial(client.Options{HostPort: c.TemporalAddress, Namespace: c.TemporalNamespace})
	if err != nil {
		return err
	}
	defer connection.Close()
	w := worker.New(connection, c.TemporalTaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(workflows.Foundation, workflow.RegisterOptions{Name: workflows.FoundationWorkflowName})
	w.RegisterActivityWithOptions(workflows.Probe, activity.RegisterOptions{Name: workflows.ProbeActivityName})
	if c.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		store, err := persistence.Open(ctx, c.DatabaseURL)
		if err != nil {
			return err
		}
		defer store.Pool.Close()
		activities := &workflows.ProbeActivities{Store: store}
		w.RegisterWorkflowWithOptions(workflows.PersistedProbe, workflow.RegisterOptions{Name: workflows.PersistedProbeWorkflowName})
		w.RegisterActivityWithOptions(activities.Complete, activity.RegisterOptions{Name: workflows.CompleteProbeActivityName})
		if c.OwnerToken != "" {
			service, err := studio.FromConfig(ctx, c, store)
			if err != nil {
				return err
			}
			media := &workflows.ImageActivities{Service: service}
			w.RegisterWorkflowWithOptions(workflows.Image, workflow.RegisterOptions{Name: workflows.ImageWorkflowName})
			w.RegisterActivityWithOptions(media.Submit, activity.RegisterOptions{Name: workflows.SubmitImageActivity})
			w.RegisterActivityWithOptions(media.Poll, activity.RegisterOptions{Name: workflows.PollImageActivity})
			w.RegisterActivityWithOptions(media.Reconcile, activity.RegisterOptions{Name: workflows.ReconcileImageActivity})
		}
	}
	return w.Run(worker.InterruptCh())
}
