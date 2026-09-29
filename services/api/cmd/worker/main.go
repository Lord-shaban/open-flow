package main

import (
	"log"
	"os"

	"github.com/Lord-shaban/open-flow/services/api/internal/config"
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
	return w.Run(worker.InterruptCh())
}
