package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/workflows"
	"go.temporal.io/sdk/client"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	connection, err := client.DialContext(ctx, client.Options{HostPort: c.TemporalAddress, Namespace: c.TemporalNamespace})
	if err != nil {
		return err
	}
	defer connection.Close()
	id := "foundation-" + rand.Text()
	run, err := connection.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: id, TaskQueue: c.TemporalTaskQueue, WorkflowExecutionTimeout: time.Minute,
	}, workflows.FoundationWorkflowName, workflows.ProbeInput{CorrelationID: id})
	if err != nil {
		return err
	}
	var result workflows.ProbeResult
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
