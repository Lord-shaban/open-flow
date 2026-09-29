package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
)

type Config struct {
	HTTPAddr          string
	LogLevel          slog.Level
	TemporalAddress   string
	TemporalNamespace string
	TemporalTaskQueue string
	KafkaBroker       string
	DatabaseURL       string
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:          env("OPEN_FLOW_HTTP_ADDR", "127.0.0.1:8080"),
		TemporalAddress:   env("OPEN_FLOW_TEMPORAL_ADDRESS", "127.0.0.1:7233"),
		TemporalNamespace: env("OPEN_FLOW_TEMPORAL_NAMESPACE", "default"),
		TemporalTaskQueue: env("OPEN_FLOW_TEMPORAL_TASK_QUEUE", "open-flow-media"),
		KafkaBroker:       env("OPEN_FLOW_KAFKA_BROKER", "127.0.0.1:9092"),
		DatabaseURL:       os.Getenv("OPEN_FLOW_DATABASE_URL"),
	}
	for name, address := range map[string]string{"HTTP": c.HTTPAddr, "Temporal": c.TemporalAddress, "Kafka": c.KafkaBroker} {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return Config{}, fmt.Errorf("%s address must be host:port: %w", name, err)
		}
	}
	if err := c.LogLevel.UnmarshalText([]byte(env("OPEN_FLOW_LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("invalid OPEN_FLOW_LOG_LEVEL: %w", err)
	}
	return c, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
