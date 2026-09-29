package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	down := len(args) == 2 && args[0] == "down" && args[1] == "--allow-data-loss"
	if !down && !(len(args) == 1 && args[0] == "up") {
		return errors.New("usage: migrate up | migrate down --allow-data-loss (drops pipeline tables)")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := persistence.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Pool.Close()
	if err = store.Migrate(ctx, down); err != nil {
		return errors.New("migration failed; check schema/version compatibility")
	}
	log.Print("migration complete")
	return nil
}
