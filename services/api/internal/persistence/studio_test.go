package persistence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"sync"
	"testing"
	"time"
)

func TestCredentialIsolationRotationAndRevocation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner, other := uuid.NewString(), uuid.NewString()
	_ = s.EnsureOwner(ctx, owner)
	_ = s.EnsureOwner(ctx, other)
	c := Credential{ID: uuid.NewString(), OwnerID: owner, Name: "fixture", Provider: "horde", Ciphertext: bytes.Repeat([]byte{1}, 40), Version: 1}
	if err := s.SaveCredential(ctx, c, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Credential(ctx, other, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-owner read")
	}
	wrong := c
	wrong.OwnerID = other
	if err := s.SaveCredential(ctx, wrong, true); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-owner rotate")
	}
	if err := s.RevokeCredential(ctx, other, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-owner revoke")
	}
	c.Version = 2
	c.Ciphertext = bytes.Repeat([]byte{2}, 40)
	if err := s.SaveCredential(ctx, c, true); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Credential(ctx, owner, c.ID)
	if err != nil || saved.Revision != 2 || saved.Version != 2 {
		t.Fatal(saved, err)
	}
	encoded, _ := json.Marshal(saved)
	if bytes.Contains(encoded, []byte("Ciphertext")) || bytes.Contains(encoded, []byte("encrypted_payload")) {
		t.Fatal("ciphertext exposed")
	}
	if err = s.RevokeCredential(ctx, owner, c.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveCredential(ctx, c, true); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked credential rotated")
	}
}
func imageInput(t *testing.T, s *Store, owner string) NewGeneration {
	t.Helper()
	if err := s.EnsureOwner(context.Background(), owner); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("fixture"))
	return NewGeneration{ID: uuid.NewString(), Owner: owner, Provider: "training", Model: "training-landscape-v1", Aspect: "1:1", Idempotency: uuid.NewString(), Hash: hash[:], EncryptedPrompt: bytes.Repeat([]byte{3}, 40), Version: 1}
}
func TestImageIdempotencyIntentAndAtomicArtifacts(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	n := imageInput(t, s, uuid.NewString())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			copy := n
			copy.ID = uuid.NewString()
			id, _, err := s.CreateGeneration(ctx, copy)
			if err != nil {
				t.Error(err)
			}
			if id == "" {
				t.Error("missing identity")
			}
		})
	}
	wg.Wait()
	var id string
	var count int
	if err := s.Pool.QueryRow(ctx, `SELECT id::text FROM jobs WHERE owner_id=$1 AND idempotency_key=$2`, n.Owner, n.Idempotency).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if replay, err := s.GenerationForKey(ctx, n.Owner, n.Idempotency, n.Hash); err != nil || replay.ID != id {
		t.Fatal("accepted replay unavailable", err)
	}
	if _, err := s.GenerationForKey(ctx, n.Owner, n.Idempotency, bytes.Repeat([]byte{7}, 32)); !errors.Is(err, ErrConflict) {
		t.Fatal("replayed changed payload")
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE aggregate_id=$1`, id).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	conflict := n
	conflict.Hash = bytes.Repeat([]byte{5}, 32)
	if _, _, err := s.CreateGeneration(ctx, conflict); !errors.Is(err, ErrConflict) {
		t.Fatal("payload conflict accepted")
	}
	_, claimed, err := s.ClaimSubmission(ctx, id)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	if _, claimed, err = s.ClaimSubmission(ctx, id); err != nil || claimed {
		t.Fatal("duplicate submission intent")
	}
	if err = s.AcceptOperation(ctx, id, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	a := Artifact{ID: uuid.NewString(), OwnerID: n.Owner, JobID: id, Key: "open-flow/" + n.Owner + "/" + id + "/" + uuid.NewString(), Bytes: 99, ContentType: "image/png", ExpiresAt: time.Now().Add(time.Hour)}
	if err = s.Finish(ctx, id, "succeeded", "", []Artifact{a}); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, id, "succeeded", "", []Artifact{a}); err != nil {
		t.Fatal("lost response was not idempotent", err)
	}
	g, err := s.Generation(ctx, n.Owner, id)
	if err != nil || g.State != "succeeded" || len(g.Artifacts) != 1 {
		t.Fatal(g, err)
	}
	if _, err = s.Artifact(ctx, uuid.NewString(), a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-owner artifact read")
	}
	if err = s.DeleteGeneration(ctx, n.Owner, id); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Artifact(ctx, n.Owner, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted artifact still visible")
	}
	refs, err := s.ReferencedKeys(ctx)
	if err != nil || refs[a.Key] {
		t.Fatal("tombstoned reference retained", err)
	}
}
func TestOwnedCursorPaginationAndExpiry(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	owner := uuid.NewString()
	var last string
	for range 4 {
		n := imageInput(t, s, owner)
		id, _, err := s.CreateGeneration(ctx, n)
		if err != nil {
			t.Fatal(err)
		}
		last = id
	}
	page, cursor, err := s.Generations(ctx, owner, "", 2)
	if err != nil || len(page) != 2 || cursor == "" {
		t.Fatal(page, cursor, err)
	}
	next, after, err := s.Generations(ctx, owner, cursor, 2)
	if err != nil || len(next) != 2 || after != "" {
		t.Fatal(next, after, err)
	}
	if page[0].ID == next[0].ID {
		t.Fatal("repeated page")
	}
	if _, _, err = s.Generations(ctx, uuid.NewString(), last, 2); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign cursor accepted")
	}
	if err = s.DeleteGeneration(ctx, owner, last); !errors.Is(err, ErrConflict) {
		t.Fatal("queued job deletion accepted")
	}
	if _, _, err = s.ClaimSubmission(ctx, last); err != nil {
		t.Fatal(err)
	}
	expired := Artifact{ID: uuid.NewString(), JobID: last, OwnerID: owner, Key: "open-flow/" + owner + "/" + last + "/" + uuid.NewString(), ContentType: "image/png", Bytes: 99, ExpiresAt: time.Now().Add(-time.Hour)}
	if err = s.Finish(ctx, last, "succeeded", "", []Artifact{expired}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Artifact(ctx, owner, expired.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired artifact download allowed")
	}
	refs, err := s.ReferencedKeys(ctx)
	if err != nil || refs[expired.Key] {
		t.Fatal("expired artifact retained", err)
	}
}
