-- +goose Up
-- Lengths match the domain limits in internal/domain/user.go.
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(254) NOT NULL CHECK (email = lower(email)),
    full_name     VARCHAR(100) NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('applicant', 'employer', 'admin')),
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_key ON users (email);
-- Serves ListUsersByRole, which pages by role ordered by email.
CREATE INDEX users_role_email_idx ON users (role, email);

-- +goose Down
DROP TABLE users;
