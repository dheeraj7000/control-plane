CREATE TABLE IF NOT EXISTS budget_ledgers (
    scope               TEXT NOT NULL,
    owner_id            TEXT NOT NULL,
    period_key          TEXT NOT NULL DEFAULT '',
    input_tokens_limit  BIGINT NOT NULL DEFAULT 0,
    output_tokens_limit BIGINT NOT NULL DEFAULT 0,
    cost_limit_micros   BIGINT NOT NULL DEFAULT 0,
    input_tokens_used   BIGINT NOT NULL DEFAULT 0,
    output_tokens_used  BIGINT NOT NULL DEFAULT 0,
    cost_used_micros    BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (scope, owner_id, period_key)
);
