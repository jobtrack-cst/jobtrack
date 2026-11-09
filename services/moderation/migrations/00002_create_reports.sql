-- +goose Up
-- Lengths match the domain limits in internal/domain/report.go. VARCHAR(n)
-- counts characters, like the domain.
CREATE TABLE reports (
    id          UUID PRIMARY KEY,
    -- Job IDs are not checked against job-posting: moderation has no allowed
    -- call to it (AGENTS.md Section 6).
    job_id      UUID NOT NULL,
    -- The reporter's user ID from auth (x-user-id).
    reporter_id UUID NOT NULL,
    reason      VARCHAR(1000) NOT NULL CHECK (reason <> ''),
    status      TEXT NOT NULL CHECK (status IN ('open', 'dismissed', 'actioned')),
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

-- At most one OPEN report per reporter and job, even when two reports race.
-- Its leading job_id column also serves RemoveJob's update of OPEN reports.
CREATE UNIQUE INDEX reports_one_open_per_reporter
    ON reports (job_id, reporter_id) WHERE status = 'open';

-- ListReports, newest first, optionally by status.
CREATE INDEX reports_created ON reports (created_at DESC, id DESC);
CREATE INDEX reports_status_created ON reports (status, created_at DESC, id DESC);

-- Jobs moderation has removed. The primary key makes a second takedown
-- impossible, and reason and removed_by are kept for moderation.job_removed
-- in Sprint 2.
CREATE TABLE removed_jobs (
    job_id     UUID PRIMARY KEY,
    reason     VARCHAR(500) NOT NULL CHECK (reason <> ''),
    -- The admin's user ID from auth.
    removed_by UUID NOT NULL,
    removed_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE removed_jobs;
DROP TABLE reports;
