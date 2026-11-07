-- +goose Up
-- Lengths match the domain limits in internal/domain/profile.go. VARCHAR(n)
-- counts characters, like the domain. The primary key enforces one profile
-- per employer.
CREATE TABLE company_profiles (
    employer_id         UUID PRIMARY KEY,
    company_name        VARCHAR(150) NOT NULL CHECK (company_name <> ''),
    description         VARCHAR(2000) NOT NULL DEFAULT '',
    website             VARCHAR(255) NOT NULL DEFAULT '',
    location            VARCHAR(100) NOT NULL DEFAULT '',
    verification_status TEXT NOT NULL DEFAULT 'unverified'
        CHECK (verification_status IN ('unverified', 'verified', 'rejected')),
    -- Set only by the employer.verification_rejected consumer (Sprint 2).
    rejection_reason    TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE company_profiles;
