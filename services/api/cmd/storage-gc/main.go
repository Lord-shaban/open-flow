package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/studio"
	"log"
	"os"
	"time"
)

func main() {
	apply := flag.Bool("apply", false, "Delete unreferenced objects; stop the image worker first")
	flag.Parse()
	if err := run(*apply); err != nil {
		log.Print("storage cleanup failed")
		os.Exit(1)
	}
}
func run(apply bool) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	store, err := persistence.Open(ctx, c.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Pool.Close()
	s, err := studio.FromConfig(ctx, c, store)
	if err != nil {
		return err
	}
	refs, err := store.ReferencedKeys(ctx)
	if err != nil {
		return err
	}
	count, err := s.Storage.Sweep(ctx, refs, time.Now().Add(-24*time.Hour), apply)
	if err != nil {
		return err
	}
	fmt.Printf("eligible_objects=%d apply=%t\n", count, apply)
	return nil
}
