CREATE TABLE owners (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE credentials (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES owners(id),
    provider text NOT NULL CHECK (length(provider) BETWEEN 1 AND 80),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    encrypted_payload bytea NOT NULL CHECK (octet_length(encrypted_payload) >= 28),
    key_version integer NOT NULL CHECK (key_version > 0),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, owner_id)
);
CREATE INDEX credentials_owner_idx ON credentials(owner_id);

CREATE TABLE jobs (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES owners(id),
    kind text NOT NULL CHECK (kind IN ('foundation_probe', 'image', 'video')),
    state text NOT NULL DEFAULT 'queued' CHECK (state IN
        ('queued', 'submitting', 'running', 'reconciliation_required', 'cancel_requested', 'succeeded', 'failed', 'canceled')),
    credential_id uuid,
    payload_ref text,
    idempotency_key text CHECK (length(idempotency_key) BETWEEN 1 AND 200),
    request_hash bytea CHECK (octet_length(request_hash) = 32),
    workflow_id text NOT NULL UNIQUE CHECK (workflow_id = 'open-flow.job.' || id::text),
    dispatch_state text NOT NULL DEFAULT 'pending' CHECK (dispatch_state IN ('pending', 'started')),
    dispatch_attempts integer NOT NULL DEFAULT 0 CHECK (dispatch_attempts >= 0),
    dispatch_next_at timestamptz NOT NULL DEFAULT now(),
    workflow_run_id text,
    sequence bigint NOT NULL DEFAULT 1 CHECK (sequence > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, owner_id),
    UNIQUE (owner_id, idempotency_key),
    FOREIGN KEY (credential_id, owner_id) REFERENCES credentials(id, owner_id),
    CHECK ((idempotency_key IS NULL) = (request_hash IS NULL)),
    CHECK ((dispatch_state = 'started') = (workflow_run_id IS NOT NULL))
);
CREATE INDEX jobs_owner_created_idx ON jobs(owner_id, created_at DESC, id);
CREATE INDEX jobs_credential_idx ON jobs(credential_id, owner_id) WHERE credential_id IS NOT NULL;
CREATE INDEX jobs_dispatch_idx ON jobs(dispatch_next_at, created_at, id) WHERE dispatch_state = 'pending';

CREATE TABLE attempts (
    id uuid PRIMARY KEY,
    job_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    attempt_number integer NOT NULL CHECK (attempt_number > 0),
    state text NOT NULL CHECK (state IN ('started', 'accepted', 'succeeded', 'failed', 'ambiguous')),
    provider text NOT NULL,
    operation_ref text,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (job_id, owner_id) REFERENCES jobs(id, owner_id),
    UNIQUE (job_id, attempt_number)
);
CREATE INDEX attempts_owner_idx ON attempts(owner_id);

CREATE TABLE artifacts (
    id uuid PRIMARY KEY,
    job_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    storage_key text NOT NULL UNIQUE,
    content_type text NOT NULL,
    byte_size bigint NOT NULL CHECK (byte_size > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (job_id, owner_id) REFERENCES jobs(id, owner_id)
);
CREATE INDEX artifacts_job_idx ON artifacts(job_id, owner_id);
CREATE INDEX artifacts_owner_idx ON artifacts(owner_id);

CREATE TABLE usage (
    id uuid PRIMARY KEY,
    job_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    source_event_id uuid NOT NULL UNIQUE,
    units numeric(20,6) NOT NULL CHECK (units >= 0),
    unit_type text NOT NULL,
    cost numeric(20,8) CHECK (cost >= 0),
    currency text CHECK (currency ~ '^[A-Z]{3}$'),
    estimated boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (job_id, owner_id) REFERENCES jobs(id, owner_id),
    CHECK ((cost IS NULL) = (currency IS NULL))
);
CREATE INDEX usage_job_idx ON usage(job_id, owner_id);
CREATE INDEX usage_owner_created_idx ON usage(owner_id, created_at DESC);

CREATE TABLE outbox (
    event_id uuid PRIMARY KEY,
    aggregate_id uuid NOT NULL REFERENCES jobs(id),
    sequence bigint NOT NULL CHECK (sequence > 0),
    event_type text NOT NULL CHECK (event_type IN ('job.queued', 'job.succeeded')),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    publish_attempts integer NOT NULL DEFAULT 0 CHECK (publish_attempts >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    UNIQUE (aggregate_id, sequence)
);
CREATE INDEX outbox_ready_idx ON outbox(next_attempt_at, occurred_at, event_id) WHERE published_at IS NULL;
CREATE INDEX outbox_aggregate_pending_idx ON outbox(aggregate_id, sequence) WHERE published_at IS NULL;

CREATE TABLE consumer_inbox (
    consumer_name text NOT NULL CHECK (length(consumer_name) BETWEEN 1 AND 120),
    event_id uuid NOT NULL,
    aggregate_id uuid NOT NULL REFERENCES jobs(id),
    sequence bigint NOT NULL CHECK (sequence > 0),
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer_name, event_id)
);
CREATE INDEX consumer_inbox_aggregate_idx ON consumer_inbox(aggregate_id);

-- Training projection, not a billing ledger. The actual usage consumer is OF-013.
CREATE TABLE job_event_projections (
    consumer_name text NOT NULL,
    job_id uuid NOT NULL REFERENCES jobs(id),
    last_sequence bigint NOT NULL CHECK (last_sequence > 0),
    last_type text NOT NULL CHECK (last_type IN ('job.queued', 'job.succeeded')),
    applied_count bigint NOT NULL DEFAULT 1 CHECK (applied_count > 0),
    PRIMARY KEY (consumer_name, job_id)
);
CREATE INDEX job_event_projections_job_idx ON job_event_projections(job_id);
