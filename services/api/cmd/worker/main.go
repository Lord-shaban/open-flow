package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
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
	}
	return w.Run(worker.InterruptCh())
}
