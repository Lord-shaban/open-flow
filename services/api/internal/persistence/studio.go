package persistence

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Credential struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Provider   string    `json:"provider"`
	Status     string    `json:"status"`
	Revision   int       `json:"revision"`
	CreatedAt  time.Time `json:"created_at"`
	OwnerID    string    `json:"-"`
	Ciphertext []byte    `json:"-"`
	Version    int       `json:"-"`
}

func (s *Store) EnsureOwner(ctx context.Context, owner string) error {
	if !validID(owner) {
		return ErrInvalid
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO owners(id,name) VALUES ($1,'Open Flow owner') ON CONFLICT DO NOTHING`, owner)
	return err
}
func (s *Store) SaveCredential(ctx context.Context, c Credential, rotate bool) error {
	if !validID(c.ID) || !validID(c.OwnerID) || (c.Provider != "gemini" && c.Provider != "cloudflare" && c.Provider != "horde") || len(c.Name) < 1 || len(c.Name) > 120 || len(c.Ciphertext) < 28 || c.Version < 1 {
		return ErrInvalid
	}
	if rotate {
		tag, err := s.Pool.Exec(ctx, `UPDATE credentials SET encrypted_payload=$3,key_version=$4,revision=revision+1,name=$5 WHERE id=$1 AND owner_id=$2 AND status='active' AND provider=$6`, c.ID, c.OwnerID, c.Ciphertext, c.Version, c.Name, c.Provider)
		if err == nil && tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return err
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO credentials(id,owner_id,provider,name,encrypted_payload,key_version) VALUES ($1,$2,$3,$4,$5,$6)`, c.ID, c.OwnerID, c.Provider, c.Name, c.Ciphertext, c.Version)
	return err
}
func (s *Store) Credential(ctx context.Context, owner, id string) (Credential, error) {
	if !validID(owner) || !validID(id) {
		return Credential{}, ErrInvalid
	}
	var c Credential
	err := s.Pool.QueryRow(ctx, `SELECT id::text,owner_id::text,provider,name,status,encrypted_payload,key_version,revision,created_at FROM credentials WHERE id=$1 AND owner_id=$2`, id, owner).Scan(&c.ID, &c.OwnerID, &c.Provider, &c.Name, &c.Status, &c.Ciphertext, &c.Version, &c.Revision, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}
func (s *Store) Credentials(ctx context.Context, owner string) ([]Credential, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text,provider,name,status,revision,created_at FROM credentials WHERE owner_id=$1 ORDER BY created_at DESC LIMIT 100`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Credential{}
	for rows.Next() {
		var c Credential
		if err := rows.Scan(&c.ID, &c.Provider, &c.Name, &c.Status, &c.Revision, &c.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
func (s *Store) RevokeCredential(ctx context.Context, owner, id string) error {
	if !validID(owner) || !validID(id) {
		return ErrInvalid
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE credentials SET status='revoked',revision=revision+1 WHERE id=$1 AND owner_id=$2`, id, owner)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

type Artifact struct {
	ID          string    `json:"id"`
	JobID       string    `json:"job_id"`
	ContentType string    `json:"content_type"`
	Bytes       int64     `json:"bytes"`
	ExpiresAt   time.Time `json:"expires_at"`
	OwnerID     string    `json:"-"`
	Key         string    `json:"-"`
}
type Generation struct {
	ID           string     `json:"id"`
	State        string     `json:"state"`
	Provider     string     `json:"provider"`
	Model        string     `json:"model_id"`
	AspectRatio  string     `json:"aspect_ratio"`
	CredentialID string     `json:"credential_id,omitempty"`
	ErrorCode    string     `json:"error_code,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Artifacts    []Artifact `json:"artifacts"`
	RouteReason  string     `json:"route_reason"`
	OwnerID      string     `json:"-"`
	Ciphertext   []byte     `json:"-"`
	Version      int        `json:"-"`
	Operation    string     `json:"-"`
}
type NewGeneration struct {
	ID, Owner, Provider, Model, Aspect, Credential, Idempotency string
	Hash, EncryptedPrompt                                       []byte
	Version                                                     int
}

func (s *Store) CreateGeneration(ctx context.Context, n NewGeneration) (string, bool, error) {
	if !validID(n.ID) || !validID(n.Owner) || len(n.Idempotency) < 1 || len(n.Idempotency) > 200 || len(n.Hash) != 32 || len(n.EncryptedPrompt) < 28 {
		return "", false, ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer rollback(tx)
	// A credential revoke/rotation serializes with acceptance; queued activities recheck it.
	if n.Credential != "" {
		var active bool
		err = tx.QueryRow(ctx, `SELECT status='active' FROM credentials WHERE id=$1 AND owner_id=$2 FOR SHARE`, n.Credential, n.Owner).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
			return "", false, ErrNotFound
		}
		if err != nil {
			return "", false, err
		}
	}
	tag, err := tx.Exec(ctx, `INSERT INTO jobs(id,owner_id,kind,workflow_id,credential_id,idempotency_key,request_hash,payload_ref,provider,model_id,aspect_ratio) VALUES ($1,$2,'image',$3,NULLIF($4,'')::uuid,$5,$6,$1::text,$7,$8,$9) ON CONFLICT (owner_id,idempotency_key) DO NOTHING`, n.ID, n.Owner, WorkflowID(n.ID), n.Credential, n.Idempotency, n.Hash, n.Provider, n.Model, n.Aspect)
	if err != nil {
		return "", false, err
	}
	if tag.RowsAffected() == 0 {
		var id string
		var hash []byte
		err = tx.QueryRow(ctx, `SELECT id::text,request_hash FROM jobs WHERE owner_id=$1 AND idempotency_key=$2`, n.Owner, n.Idempotency).Scan(&id, &hash)
		if err != nil {
			return "", false, err
		}
		if subtle.ConstantTimeCompare(n.Hash, hash) != 1 {
			return "", false, ErrConflict
		}
		return id, false, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `INSERT INTO job_inputs(job_id,owner_id,encrypted_payload,key_version) VALUES ($1,$2,$3,$4)`, n.ID, n.Owner, n.EncryptedPrompt, n.Version)
	if err != nil {
		return "", false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox(event_id,aggregate_id,sequence,event_type) VALUES ($1,$2,1,'job.queued')`, uuid.NewString(), n.ID)
	if err != nil {
		return "", false, err
	}
	return n.ID, true, tx.Commit(ctx)
}

// An accepted idempotency key remains replayable without contacting its provider.
func (s *Store) GenerationForKey(ctx context.Context, owner, key string, hash []byte) (Generation, error) {
	var id string
	var stored []byte
	err := s.Pool.QueryRow(ctx, `SELECT id::text,request_hash FROM jobs WHERE owner_id=$1 AND idempotency_key=$2 AND kind='image'`, owner, key).Scan(&id, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return Generation{}, ErrNotFound
	}
	if err != nil {
		return Generation{}, err
	}
	if subtle.ConstantTimeCompare(hash, stored) != 1 {
		return Generation{}, ErrConflict
	}
	return s.Generation(ctx, owner, id)
}

func scanGeneration(row pgx.Row) (Generation, error) {
	var g Generation
	g.Artifacts = []Artifact{}
	err := row.Scan(&g.ID, &g.OwnerID, &g.State, &g.Provider, &g.Model, &g.AspectRatio, &g.CredentialID, &g.ErrorCode, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, ErrNotFound
	}
	g.RouteReason = "Selected explicitly; no automatic provider or credential failover."
	if g.Provider == "horde" {
		g.RouteReason = "AI Horde volunteer queue. Anonymous generations may be shared; no account or card required."
	}
	if g.Provider == "cloudflare" {
		g.RouteReason = "Explicit Workers AI Free connection. Account-wide daily allowance; no automatic upgrade or failover."
	}
	if g.Provider == "training" {
		g.RouteReason = "Local training renderer. No AI inference, account, network provider, or charges."
	}
	if g.Provider == "comfyui" {
		g.RouteReason = "Local ComfyUI inference using your installed checkpoint. No hosted service."
	}
	return g, err
}

const generationColumns = `id::text,owner_id::text,state,provider,model_id,aspect_ratio,COALESCE(credential_id::text,''),COALESCE(error_code,''),created_at,updated_at`

func (s *Store) Generation(ctx context.Context, owner, id string) (Generation, error) {
	if !validID(owner) || !validID(id) {
		return Generation{}, ErrInvalid
	}
	g, err := scanGeneration(s.Pool.QueryRow(ctx, `SELECT `+generationColumns+` FROM jobs WHERE id=$1 AND owner_id=$2 AND kind='image' AND deleted_at IS NULL`, id, owner))
	if err != nil {
		return g, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text,job_id::text,content_type,byte_size,expires_at FROM artifacts WHERE job_id=$1 AND owner_id=$2 AND deleted_at IS NULL AND expires_at>now() ORDER BY created_at`, id, owner)
	if err != nil {
		return g, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Artifact
		if err = rows.Scan(&a.ID, &a.JobID, &a.ContentType, &a.Bytes, &a.ExpiresAt); err != nil {
			return g, err
		}
		g.Artifacts = append(g.Artifacts, a)
	}
	return g, rows.Err()
}

// Cursor is a UUID anchor, scoped to the owner; ordering uses immutable (created_at,id).
func (s *Store) Generations(ctx context.Context, owner, cursor string, limit int) ([]Generation, string, error) {
	if !validID(owner) || limit < 1 || limit > 50 || cursor != "" && !validID(cursor) {
		return nil, "", ErrInvalid
	}
	var at any
	var anchor any
	if cursor != "" {
		var stamp time.Time
		err := s.Pool.QueryRow(ctx, `SELECT created_at FROM jobs WHERE id=$1 AND owner_id=$2 AND kind='image' AND deleted_at IS NULL`, cursor, owner).Scan(&stamp)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		if err != nil {
			return nil, "", err
		}
		at = stamp
		anchor = cursor
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text FROM jobs WHERE owner_id=$1 AND kind='image' AND deleted_at IS NULL AND ($2::timestamptz IS NULL OR (created_at,id)<($2::timestamptz,$3::uuid)) ORDER BY created_at DESC,id DESC LIMIT $4`, owner, at, anchor, limit+1)
	if err != nil {
		return nil, "", err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, "", err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(ids) > limit {
		ids = ids[:limit]
		next = ids[len(ids)-1]
	}
	result := []Generation{}
	for _, id := range ids {
		g, e := s.Generation(ctx, owner, id)
		if e != nil {
			return nil, "", e
		}
		result = append(result, g)
	}
	return result, next, nil
}

// ClaimSubmission commits a once-only submit intent before any external side effect.
// A repeated claim cannot submit again; accepted operations can instead be polled.
func (s *Store) ClaimSubmission(ctx context.Context, id string) (Generation, bool, error) {
	if !validID(id) {
		return Generation{}, false, ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Generation{}, false, err
	}
	defer rollback(tx)
	g, err := scanGeneration(tx.QueryRow(ctx, `SELECT `+generationColumns+` FROM jobs WHERE id=$1 AND kind='image' AND deleted_at IS NULL FOR UPDATE`, id))
	if err != nil {
		return g, false, err
	}
	if g.State != "queued" {
		if g.State == "running" {
			err = tx.QueryRow(ctx, `SELECT COALESCE(operation_ref,'') FROM attempts WHERE job_id=$1 AND attempt_number=1`, id).Scan(&g.Operation)
			if err != nil {
				return g, false, err
			}
		}
		return g, false, tx.Commit(ctx)
	}
	err = tx.QueryRow(ctx, `SELECT encrypted_payload,key_version FROM job_inputs WHERE job_id=$1 AND owner_id=$2`, id, g.OwnerID).Scan(&g.Ciphertext, &g.Version)
	if err != nil {
		return g, false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO attempts(id,job_id,owner_id,attempt_number,state,provider) VALUES ($1,$2,$3,1,'started',$4)`, uuid.NewString(), id, g.OwnerID, g.Provider)
	if err != nil {
		return g, false, err
	}
	if err = transition(ctx, tx, id, "submitting", ""); err != nil {
		return g, false, err
	}
	g.State = "submitting"
	return g, true, tx.Commit(ctx)
}
func transition(ctx context.Context, tx pgx.Tx, id, state, code string) error {
	var sequence int64
	err := tx.QueryRow(ctx, `UPDATE jobs SET state=$2,error_code=NULLIF($3,''),sequence=sequence+1,updated_at=now() WHERE id=$1 RETURNING sequence`, id, state, code).Scan(&sequence)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox(event_id,aggregate_id,sequence,event_type) VALUES ($1,$2,$3,$4)`, uuid.NewString(), id, sequence, "job."+state)
	return err
}
func (s *Store) AcceptOperation(ctx context.Context, id, op string) error {
	if op == "" || len(op) > 200 {
		return ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM jobs WHERE id=$1 FOR UPDATE`, id).Scan(&state); err != nil {
		return err
	}
	if state == "running" {
		return tx.Commit(ctx)
	}
	if state != "submitting" {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE attempts SET state='accepted',operation_ref=$2 WHERE job_id=$1 AND attempt_number=1`, id, op)
	if err != nil {
		return err
	}
	if err = transition(ctx, tx, id, "running", ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Finish commits private artifacts, terminal state and lifecycle event atomically.
func (s *Store) Finish(ctx context.Context, id, state, code string, artifacts []Artifact) error {
	if state != "succeeded" && state != "failed" && state != "reconciliation_required" {
		return ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var current, owner string
	if err = tx.QueryRow(ctx, `SELECT state,owner_id::text FROM jobs WHERE id=$1 AND kind='image' FOR UPDATE`, id).Scan(&current, &owner); err != nil {
		return err
	}
	if current == "succeeded" || current == "failed" || current == "reconciliation_required" {
		return tx.Commit(ctx)
	}
	for _, a := range artifacts {
		if a.OwnerID != owner || a.JobID != id || !validID(a.ID) || a.Bytes < 1 {
			return ErrInvalid
		}
		_, err = tx.Exec(ctx, `INSERT INTO artifacts(id,job_id,owner_id,storage_key,content_type,byte_size,expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, a.ID, id, owner, a.Key, a.ContentType, a.Bytes, a.ExpiresAt)
		if err != nil {
			return err
		}
	}
	if state == "succeeded" && len(artifacts) == 0 {
		return ErrInvalid
	}
	attemptState := state
	if state == "reconciliation_required" {
		attemptState = "ambiguous"
	}
	_, err = tx.Exec(ctx, `UPDATE attempts SET state=$2,error_code=NULLIF($3,'') WHERE job_id=$1 AND attempt_number=1`, id, attemptState, code)
	if err != nil {
		return err
	}
	if err = transition(ctx, tx, id, state, code); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) Artifact(ctx context.Context, owner, id string) (Artifact, error) {
	if !validID(owner) || !validID(id) {
		return Artifact{}, ErrInvalid
	}
	var a Artifact
	err := s.Pool.QueryRow(ctx, `SELECT a.id::text,a.job_id::text,a.owner_id::text,a.storage_key,a.content_type,a.byte_size,a.expires_at FROM artifacts a JOIN jobs j ON j.id=a.job_id WHERE a.id=$1 AND a.owner_id=$2 AND a.deleted_at IS NULL AND j.deleted_at IS NULL AND a.expires_at>now()`, id, owner).Scan(&a.ID, &a.JobID, &a.OwnerID, &a.Key, &a.ContentType, &a.Bytes, &a.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// Retention tombstones first, so every new download is denied before GC runs.
func (s *Store) DeleteGeneration(ctx context.Context, owner, id string) error {
	g, err := s.Generation(ctx, owner, id)
	if err != nil {
		return err
	}
	if g.State != "succeeded" && g.State != "failed" && g.State != "reconciliation_required" {
		return ErrConflict
	}
	_, err = s.Pool.Exec(ctx, `UPDATE jobs SET deleted_at=now() WHERE id=$1 AND owner_id=$2 AND state IN ('succeeded','failed','reconciliation_required')`, id, owner)
	return err
}
func (s *Store) ReferencedKeys(ctx context.Context) (map[string]bool, error) {
	rows, err := s.Pool.Query(ctx, `SELECT a.storage_key FROM artifacts a JOIN jobs j ON j.id=a.job_id WHERE a.deleted_at IS NULL AND j.deleted_at IS NULL AND a.expires_at>now()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refs := map[string]bool{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		refs[key] = true
	}
	return refs, rows.Err()
}
