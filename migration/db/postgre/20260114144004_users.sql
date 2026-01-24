-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    account_number VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NULL,
    phone_number VARCHAR(17) NULL,
    phone_country_code VARCHAR(3) NOT NULL,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    CONSTRAINT check_email_or_phone CHECK (
        (
            email IS NOT NULL
            AND email != ''
        )
        OR (
            phone_number IS NOT NULL
            AND phone_number != ''
        )
    )
);

-- Create unique indexes that treat empty strings as NULL (allows multiple NULLs but prevents duplicate values)
CREATE UNIQUE INDEX idx_users_email_unique ON users (email)
WHERE
    email IS NOT NULL
    AND email != '';

CREATE UNIQUE INDEX idx_users_phone_number_unique ON users (phone_number)
WHERE
    phone_number IS NOT NULL
    AND phone_number != '';

-- Create regular indexes for performance on lookups
CREATE INDEX idx_users_email ON users (email);

CREATE INDEX idx_users_phone_number ON users (phone_number);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE users;

-- +goose StatementEnd