-- +goose Up
-- Lengths and limits match internal/domain/resume.go and the resume.proto
-- contract: PDF only, 1 to 2,000,000 bytes. VARCHAR(n) counts characters.
CREATE TABLE resumes (
    resume_id         UUID PRIMARY KEY,
    -- The applicant's user ID from auth.
    applicant_id      UUID NOT NULL,
    file_name         VARCHAR(255) NOT NULL CHECK (file_name <> ''),
    content_type      TEXT NOT NULL CHECK (content_type = 'application/pdf'),
    size_bytes        BIGINT NOT NULL CHECK (size_bytes BETWEEN 1 AND 2000000),
    content           BYTEA NOT NULL CHECK (octet_length(content) = size_bytes),
    -- VALIDATED and REJECTED are set only by the Sprint 2 event consumers.
    validation_status TEXT NOT NULL CHECK (validation_status IN ('pending', 'validated', 'rejected')),
    rejection_reason  TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL
);

-- ListMyResumes: one applicant's resumes, newest first.
CREATE INDEX resumes_applicant_created_idx ON resumes (applicant_id, created_at DESC, resume_id DESC);

-- +goose Down
DROP TABLE resumes;
