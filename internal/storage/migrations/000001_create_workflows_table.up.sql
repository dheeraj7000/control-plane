CREATE TABLE IF NOT EXISTS workflows (
    id          TEXT NOT NULL,
    version     INTEGER NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    steps       JSONB NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (id, version)
);
