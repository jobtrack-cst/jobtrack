-- +goose Up
-- Lengths match the domain limits in internal/domain/job.go. VARCHAR(n)
-- counts characters, like the domain. The database generates job IDs.
CREATE TABLE jobs (
    job_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The employer's user ID from auth.
    employer_id UUID NOT NULL,
    title       VARCHAR(150) NOT NULL CHECK (title <> ''),
    description VARCHAR(5000) NOT NULL DEFAULT '',
    category_id UUID NULL,
    tag_ids     UUID[] NOT NULL DEFAULT '{}' CHECK (cardinality(tag_ids) <= 20),
    location    VARCHAR(100) NOT NULL DEFAULT '',
    deadline    TIMESTAMPTZ NOT NULL,
    -- REMOVED is set only by the moderation.job_removed consumer (Sprint 2).
    status      TEXT NOT NULL CHECK (status IN ('draft', 'published', 'closed', 'removed')),
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

-- ListMyJobs: one employer's jobs, newest first.
CREATE INDEX jobs_employer_created_idx ON jobs (employer_id, created_at DESC, job_id DESC);

-- ListJobsClosingBetween: published jobs by deadline.
CREATE INDEX jobs_published_deadline_idx ON jobs (deadline, job_id) WHERE status = 'published';

-- +goose Down
DROP TABLE jobs;
