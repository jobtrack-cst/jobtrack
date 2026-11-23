-- +goose Up
-- JT-114 Part B: local projection of application.submitted, so the owning
-- employer can download the resume used for an application. Rows are written
-- only by the application consumer and never change. No foreign key to
-- resumes: the projection outlives a missing resume, which reads as NOT_FOUND.
CREATE TABLE application_resumes (
    application_id UUID PRIMARY KEY,
    applicant_id   UUID NOT NULL,
    employer_id    UUID NOT NULL,
    resume_id      UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE application_resumes;
