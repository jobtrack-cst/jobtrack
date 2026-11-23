-- +goose Up
-- JT-114 Part C: the first consumer in applicant-profile. Each handled
-- event_id is claimed here in the same transaction as its change.
CREATE TABLE processed_events (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Local projection of application.submitted, so the owning employer can read
-- the applicant's profile and photo. Rows are written only by the application
-- consumer and never change. No foreign key to applicant_profiles: an
-- applicant without a profile reads as NOT_FOUND.
CREATE TABLE application_participants (
    application_id UUID PRIMARY KEY,
    applicant_id   UUID NOT NULL,
    employer_id    UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE application_participants;
DROP TABLE processed_events;
