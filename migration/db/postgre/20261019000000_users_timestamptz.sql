-- +goose Up
-- Assumes each stored value is a UTC wall-clock time. That holds for every row
-- the app wrote: pgx sends a time.Time over the simple protocol as UTC text,
-- and a TIMESTAMP column drops the offset.
-- +goose StatementBegin
ALTER TABLE users
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN deleted_at TYPE TIMESTAMPTZ USING deleted_at AT TIME ZONE 'UTC';

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN deleted_at TYPE TIMESTAMP USING deleted_at AT TIME ZONE 'UTC';

-- +goose StatementEnd
