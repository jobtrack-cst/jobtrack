-- +goose Up
-- JT-103: one photo per applicant, kept apart from applicant_profiles so
-- profile reads never load the image bytes. The limits match
-- internal/domain/photo.go and the JT-102 company logo.
CREATE TABLE profile_photos (
    applicant_id UUID PRIMARY KEY REFERENCES applicant_profiles (applicant_id) ON DELETE CASCADE,
    content_type TEXT NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png')),
    content      BYTEA NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 1000000),
    updated_at   TIMESTAMPTZ NOT NULL,
    -- The file signature must match the declared type, as in the domain.
    CHECK ((content_type = 'image/jpeg' AND substring(content FROM 1 FOR 3) = decode('ffd8ff', 'hex'))
        OR (content_type = 'image/png' AND substring(content FROM 1 FOR 8) = decode('89504e470d0a1a0a', 'hex')))
);

-- +goose Down
DROP TABLE profile_photos;
