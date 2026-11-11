-- +goose Up
-- Status values match internal/domain/status.go. The lifecycle rules live in
-- the domain; these checks stop impossible rows if something bypasses it.
CREATE TABLE application_statuses (
    application_id UUID PRIMARY KEY,
    -- Trusted participants from application.submitted (Sprint 2 consumer).
    applicant_id   UUID NOT NULL,
    employer_id    UUID NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('submitted', 'under_review', 'shortlisted', 'interview',
                       'offered', 'accepted', 'declined', 'rejected', 'withdrawn')),
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL
);

CREATE TABLE status_history (
    history_id      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    application_id  UUID NOT NULL REFERENCES application_statuses (application_id) ON DELETE CASCADE,
    -- NULL only for the first entry, which is always SUBMITTED.
    previous_status TEXT NULL CHECK (previous_status IN ('submitted', 'under_review', 'shortlisted', 'interview',
                        'offered', 'accepted', 'declined', 'rejected', 'withdrawn')),
    new_status      TEXT NOT NULL CHECK (new_status IN ('submitted', 'under_review', 'shortlisted', 'interview',
                        'offered', 'accepted', 'declined', 'rejected', 'withdrawn')),
    occurred_at     TIMESTAMPTZ NOT NULL,
    CHECK ((previous_status IS NULL) = (new_status = 'submitted'))
);

-- One first entry per application.
CREATE UNIQUE INDEX status_history_first_idx ON status_history (application_id) WHERE previous_status IS NULL;

-- ListApplicationStatusHistory: one application's entries, oldest first.
CREATE INDEX status_history_application_idx ON status_history (application_id, occurred_at, history_id);

-- +goose Down
DROP TABLE status_history;
DROP TABLE application_statuses;
