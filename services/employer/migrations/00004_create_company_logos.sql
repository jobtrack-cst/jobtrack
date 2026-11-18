-- +goose Up
-- JT-102: one logo per employer, kept apart from company_profiles so profile
-- reads never load the image bytes. The limits match internal/domain/logo.go.
CREATE TABLE company_logos (
    employer_id  UUID PRIMARY KEY REFERENCES company_profiles (employer_id) ON DELETE CASCADE,
    content_type TEXT NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png')),
    content      BYTEA NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 1000000),
    updated_at   TIMESTAMPTZ NOT NULL,
    -- The file signature must match the declared type, as in the domain.
    CHECK ((content_type = 'image/jpeg' AND substring(content FROM 1 FOR 3) = decode('ffd8ff', 'hex'))
        OR (content_type = 'image/png' AND substring(content FROM 1 FOR 8) = decode('89504e470d0a1a0a', 'hex')))
);

-- +goose Down
DROP TABLE company_logos;
