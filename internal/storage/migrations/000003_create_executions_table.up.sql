CREATE TABLE IF NOT EXISTS executions (
    id               TEXT PRIMARY KEY,
    workflow_id      TEXT NOT NULL,
    workflow_version INTEGER NOT NULL,
    agent_id         TEXT NOT NULL DEFAULT '',
    state            TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL,
    history          JSONB NOT NULL DEFAULT '[]',
    steps            JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_executions_workflow_id ON executions (workflow_id);
CREATE INDEX IF NOT EXISTS idx_executions_state ON executions (state);
