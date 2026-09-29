ALTER TABLE credentials ADD COLUMN revision integer NOT NULL DEFAULT 1;
ALTER TABLE jobs ADD COLUMN provider text;
ALTER TABLE jobs ADD COLUMN model_id text;
ALTER TABLE jobs ADD COLUMN aspect_ratio text;
ALTER TABLE jobs ADD COLUMN error_code text;
ALTER TABLE jobs ADD COLUMN deleted_at timestamptz;
CREATE TABLE job_inputs (
 job_id uuid PRIMARY KEY, owner_id uuid NOT NULL,
 encrypted_payload bytea NOT NULL CHECK (octet_length(encrypted_payload) >= 28),
 key_version integer NOT NULL CHECK (key_version > 0),
 FOREIGN KEY (job_id, owner_id) REFERENCES jobs(id, owner_id)
);
CREATE INDEX job_inputs_owner_idx ON job_inputs(owner_id);
ALTER TABLE artifacts ADD COLUMN deleted_at timestamptz;
ALTER TABLE artifacts ADD COLUMN expires_at timestamptz NOT NULL DEFAULT now() + interval '30 days';
CREATE INDEX artifacts_retention_idx ON artifacts(expires_at) WHERE deleted_at IS NULL;
ALTER TABLE outbox DROP CONSTRAINT outbox_event_type_check;
ALTER TABLE outbox ADD CONSTRAINT outbox_event_type_check CHECK (event_type IN
 ('job.queued','job.submitting','job.running','job.succeeded','job.failed','job.reconciliation_required'));
ALTER TABLE job_event_projections DROP CONSTRAINT job_event_projections_last_type_check;
ALTER TABLE job_event_projections ADD CONSTRAINT job_event_projections_last_type_check CHECK (last_type IN
 ('job.queued','job.submitting','job.running','job.succeeded','job.failed','job.reconciliation_required'));
