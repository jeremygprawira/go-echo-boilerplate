-- +goose Up
-- +goose StatementBegin
CREATE TABLE revoked_tokens (
    jti VARCHAR(255) PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_revoked_tokens_expires_at ON revoked_tokens (expires_at);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE revoked_tokens;

-- +goose StatementEnd
