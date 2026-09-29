package config

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"net"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr          string
	LogLevel          slog.Level
	TemporalAddress   string
	TemporalNamespace string
	TemporalTaskQueue string
	KafkaBroker       string
	DatabaseURL       string
	OwnerToken        string
	OwnerID           string
	EncryptionKeys    string
	EncryptionVersion int
	S3Endpoint        string
	S3Bucket          string
	S3Region          string
	S3AccessKey       string
	S3SecretKey       string
	ComfyEndpoint     string
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:          env("OPEN_FLOW_HTTP_ADDR", "127.0.0.1:8080"),
		TemporalAddress:   env("OPEN_FLOW_TEMPORAL_ADDRESS", "127.0.0.1:7233"),
		TemporalNamespace: env("OPEN_FLOW_TEMPORAL_NAMESPACE", "default"),
		TemporalTaskQueue: env("OPEN_FLOW_TEMPORAL_TASK_QUEUE", "open-flow-media"),
		KafkaBroker:       env("OPEN_FLOW_KAFKA_BROKER", "127.0.0.1:9092"),
		DatabaseURL:       os.Getenv("OPEN_FLOW_DATABASE_URL"),
		OwnerToken:        os.Getenv("OPEN_FLOW_OWNER_TOKEN"),
		OwnerID:           env("OPEN_FLOW_OWNER_ID", "00000000-0000-4000-8000-000000000001"),
		EncryptionKeys:    os.Getenv("OPEN_FLOW_ENCRYPTION_KEYS"),
		S3Endpoint:        env("OPEN_FLOW_S3_ENDPOINT", "http://127.0.0.1:8333"),
		S3Bucket:          env("OPEN_FLOW_S3_BUCKET", "open-flow-private"),
		S3Region:          env("OPEN_FLOW_S3_REGION", "us-east-1"),
		S3AccessKey:       os.Getenv("OPEN_FLOW_S3_ACCESS_KEY"),
		S3SecretKey:       os.Getenv("OPEN_FLOW_S3_SECRET_KEY"),
		ComfyEndpoint:     os.Getenv("OPEN_FLOW_COMFYUI_ENDPOINT"),
	}
	var err error
	c.EncryptionVersion, err = strconv.Atoi(env("OPEN_FLOW_ENCRYPTION_VERSION", "1"))
	if err != nil || c.EncryptionVersion < 1 {
		return Config{}, fmt.Errorf("invalid encryption version")
	}
	if c.OwnerToken != "" || c.EncryptionKeys != "" {
		id, e := uuid.Parse(c.OwnerID)
		if len(c.OwnerToken) < 32 || e != nil || id == uuid.Nil || id.String() != c.OwnerID || c.EncryptionKeys == "" || c.DatabaseURL == "" || c.S3AccessKey == "" || c.S3SecretKey == "" {
			return Config{}, fmt.Errorf("studio requires owner token (32+ bytes), canonical owner UUID, encryption keys, database and private storage credentials")
		}
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
