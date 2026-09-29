package studio

import (
	"context"
	"github.com/Lord-shaban/open-flow/services/api/internal/config"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/storage"
	"github.com/Lord-shaban/open-flow/services/api/internal/vault"
)

func FromConfig(ctx context.Context, c config.Config, store *persistence.Store) (*Service, error) {
	v, err := vault.Parse(c.EncryptionKeys, c.EncryptionVersion)
	if err != nil {
		return nil, err
	}
	objects, err := storage.New(storage.Config{Endpoint: c.S3Endpoint, Bucket: c.S3Bucket, Region: c.S3Region, AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey})
	if err != nil {
		return nil, err
	}
	if err = store.EnsureOwner(ctx, c.OwnerID); err != nil {
		return nil, err
	}
	return New(store, v, objects, c.OwnerID, c.OwnerToken, c.ComfyEndpoint)
}
