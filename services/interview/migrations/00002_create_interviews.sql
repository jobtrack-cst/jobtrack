-- +goose Up
-- Trusted local projection of application.submitted, filled by the Sprint 2
-- consumer (same design as Review, Offer and Messaging). IDs are user IDs
-- from auth.
CREATE TABLE application_participants (
    application_id UUID PRIMARY KEY,
    applicant_id   UUID NOT NULL,
    employer_id    UUID NOT NULL
);

-- Several interviews per application are allowed, but not two at the same
-- time (JT-52). The "future, within 365 days" rule depends on the clock, so
-- only the domain checks it. details matches internal/domain/interview.go.
CREATE TABLE interviews (
    interview_id   UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES application_participants (application_id),
    employer_id    UUID NOT NULL,
    applicant_id   UUID NOT NULL,
    scheduled_at   TIMESTAMPTZ NOT NULL,
    details        VARCHAR(1000) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT interviews_application_time_key UNIQUE (application_id, scheduled_at)
);

-- ListInterviews: one application's interviews, soonest first. The unique
-- constraint's index already covers (application_id, scheduled_at).
CREATE INDEX interviews_application_order_idx ON interviews (application_id, scheduled_at, interview_id);

-- +goose Down
DROP TABLE interviews;
DROP TABLE application_participants;
