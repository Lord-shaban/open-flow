package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/events"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/pipeline"
	"go.temporal.io/sdk/client"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "dispatch" && os.Args[1] != "relay" && os.Args[1] != "consume") {
		return errors.New("usage: pipeline dispatch | relay | consume")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	store, err := persistence.Open(startup, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Pool.Close()
	switch os.Args[1] {
	case "dispatch":
		connection, err := client.DialContext(startup, client.Options{HostPort: cfg.TemporalAddress, Namespace: cfg.TemporalNamespace})
		if err != nil {
			return errors.New("Temporal connection failed")
		}
		defer connection.Close()
		starter := pipeline.TemporalStarter{Client: connection, TaskQueue: cfg.TemporalTaskQueue}
		return pipeline.RunLoop(ctx, "dispatch", func(ctx context.Context) (bool, error) { return store.DispatchOne(ctx, starter.Start) })
	case "relay":
		writer := pipeline.NewWriter(cfg.KafkaBroker)
		defer writer.Close()
		return pipeline.RunLoop(ctx, "relay", func(ctx context.Context) (bool, error) {
			return store.RelayOne(ctx, func(ctx context.Context, event events.Envelope) error { return events.Publish(ctx, writer, event) })
		})
	default:
		reader := pipeline.NewReader(cfg.KafkaBroker, pipeline.ProjectionConsumer)
		defer reader.Close()
		return pipeline.RunConsumer(ctx, reader, store)
	}
}
