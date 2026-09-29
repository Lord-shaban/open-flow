package persistence

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate serializes schema changes and checks already-applied migration content.
// Down is explicitly destructive; only the migration CLI or isolated tests call it.
func (s *Store) Migrate(ctx context.Context, down bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(704601001)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	versions := []int{1, 2}
	if down {
		versions = []int{2, 1}
	}
	for _, version := range versions {
		name := map[int]string{1: "pipeline", 2: "image_studio"}[version]
		upSQL, readErr := migrationFiles.ReadFile(fmt.Sprintf("migrations/%03d_%s.up.sql", version, name))
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(upSQL)
		checksum := hex.EncodeToString(sum[:])
		var applied string
		err = tx.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE version = $1`, version).Scan(&applied)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		exists := err == nil
		if exists && applied != checksum {
			return errors.New("applied migration checksum mismatch")
		}
		if (down && !exists) || (!down && exists) {
			continue
		}
		if down {
			sql, readErr := migrationFiles.ReadFile(fmt.Sprintf("migrations/%03d_%s.down.sql", version, name))
			if readErr != nil {
				return readErr
			}
			if _, err = tx.Exec(ctx, string(sql)); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
				return err
			}
		} else {
			if _, err = tx.Exec(ctx, string(upSQL)); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version, checksum) VALUES ($1, $2)`, version, checksum); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}
