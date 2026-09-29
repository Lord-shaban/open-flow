package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/events"
	"github.com/segmentio/kafka-go"
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	id := rand.Text()
	writer := &kafka.Writer{Addr: kafka.TCP(c.KafkaBroker), Topic: events.FoundationTopic, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, WriteTimeout: 10 * time.Second}
	defer writer.Close()
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{c.KafkaBroker}, Topic: events.FoundationTopic, Partition: 0, MinBytes: 1, MaxBytes: 1 << 20, MaxWait: time.Second})
	defer reader.Close()
	// The foundation topic has one partition and is created by Compose initialization.
	if err := reader.SetOffset(kafka.LastOffset); err != nil {
		return err
	}
	// Resolve last offset before publishing to avoid starting after our own message.
	conn, err := kafka.DialLeader(ctx, "tcp", c.KafkaBroker, events.FoundationTopic, 0)
	if err != nil {
		return err
	}
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		conn.Close()
		return err
	}
	offset, err := conn.ReadLastOffset()
	conn.Close()
	if err != nil {
		return err
	}
	if err := reader.SetOffset(offset); err != nil {
		return err
	}
	event := events.Envelope{SchemaVersion: 1, EventID: id, AggregateID: id, Sequence: 1, Type: "foundation.probed", OccurredAt: time.Now().UTC()}
	if err := events.Publish(ctx, writer, event); err != nil {
		return err
	}
	for {
		message, err := reader.ReadMessage(ctx)
		if err != nil {
			return fmt.Errorf("read foundation event: %w", err)
		}
		var received events.Envelope
		if err := json.Unmarshal(message.Value, &received); err != nil {
			return err
		}
		if received.EventID == id {
			return json.NewEncoder(os.Stdout).Encode(received)
		}
	}
}
