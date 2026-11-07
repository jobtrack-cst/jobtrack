-- +goose Up
CREATE TABLE applications (
 application_id uuid PRIMARY KEY,
 applicant_id uuid NOT NULL,
 job_id uuid NOT NULL,
 resume_id uuid NOT NULL,
 employer_id uuid NOT NULL,
 withdrawn boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 UNIQUE (applicant_id, job_id)
);
CREATE INDEX applications_employer_created ON applications (employer_id, created_at DESC, application_id DESC);
CREATE INDEX applications_applicant_created ON applications (applicant_id, created_at DESC, application_id DESC);

-- +goose Down
DROP TABLE applications;
