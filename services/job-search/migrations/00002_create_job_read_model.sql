-- +goose Up
-- Read model for job-search. Built only from job.published, job.updated and
-- job.closed events (the consumer arrives in Sprint 2; this schema is the
-- projection it will write). One row per job.
CREATE TABLE jobs (
    job_id      UUID PRIMARY KEY,
    employer_id UUID NOT NULL,
    title       TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 150),
    category_id UUID,
    -- Tag IDs as a text array keeps the OR filter simple. Lengths are UUIDs.
    tag_ids     TEXT[] NOT NULL DEFAULT '{}',
    location    TEXT NOT NULL DEFAULT '' CHECK (char_length(location) <= 100),
    deadline    TIMESTAMPTZ NOT NULL,
    -- closed is set by job.closed. Absence of a row never happens for a
    -- published job; unpublished jobs are not projected.
    closed      BOOLEAN NOT NULL DEFAULT FALSE,
    -- Internal bookkeeping for debugging event application order.
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- SearchJobs returns open jobs (not closed, deadline after now) sorted by
-- deadline soonest first, job_id breaking ties. This index covers the filter
-- and the sort for open jobs.
CREATE INDEX jobs_open_by_deadline_idx
    ON jobs (deadline, job_id)
    WHERE closed = FALSE;

-- Case-insensitive title match uses ILIKE '%q%', which cannot use a btree
-- index; acceptable at Sprint 1 scale. A trigram index can be added later
-- without a contract change.
CREATE INDEX jobs_category_idx ON jobs (category_id) WHERE category_id IS NOT NULL;
CREATE INDEX jobs_location_idx ON jobs (location) WHERE location <> '';

-- Idempotent-consumer table: the Sprint 2 consumer claims an event_id here
-- and applies the projection in one transaction, so redeliveries are ignored.
CREATE TABLE processed_event_id (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS processed_event_id;
DROP TABLE IF EXISTS jobs;