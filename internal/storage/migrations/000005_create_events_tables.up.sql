-- execution_sequences backs atomic, gap-free per-execution Sequence
-- assignment: an INSERT ... ON CONFLICT DO UPDATE against a single row
-- per execution_id is a well-known correct pattern for a per-key
-- counter under concurrent writers in Postgres (the ON CONFLICT path
-- takes a row lock before updating, so two concurrent Appends for the
-- same execution can't both get the same next_seq).
CREATE TABLE IF NOT EXISTS execution_sequences (
    execution_id TEXT PRIMARY KEY,
    next_seq     BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS events (
    id           TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL,
    type         TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    sequence     BIGINT NOT NULL,
    data         JSONB NOT NULL DEFAULT '{}',
    UNIQUE (execution_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_events_execution_id ON events (execution_id);
