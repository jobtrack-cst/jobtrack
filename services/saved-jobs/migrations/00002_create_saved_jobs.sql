-- +goose Up
CREATE TABLE saved_jobs (
    applicant_id UUID NOT NULL,
    job_id       UUID NOT NULL,
    saved_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (applicant_id, job_id)
);

-- For "ListMySavedJobs" (newest first)
CREATE INDEX saved_jobs_applicant_saved_at_idx ON saved_jobs (applicant_id, saved_at DESC);

-- For "ListJobSavers" (internal RPC)
CREATE INDEX saved_jobs_job_id_idx ON saved_jobs (job_id);

-- Remember jobs that were closed so we can reject saving them.
CREATE TABLE closed_jobs (
    job_id     UUID PRIMARY KEY,
    closed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE closed_jobs;
DROP TABLE saved_jobs;
