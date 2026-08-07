CREATE TABLE IF NOT EXISTS agents (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    allowed_tools JSONB NOT NULL DEFAULT '[]',
    token_hash    TEXT NOT NULL UNIQUE,
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL
);
