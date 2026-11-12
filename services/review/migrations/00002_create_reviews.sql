-- +goose Up
-- Trusted local projection of application.submitted, filled by the Sprint 2
-- consumer (same design as Offer and Messaging). employer_id is the
-- employer's user ID from auth.
CREATE TABLE application_participants (
    application_id UUID PRIMARY KEY,
    applicant_id   UUID NOT NULL,
    employer_id    UUID NOT NULL
);

-- One completed review per application (review.proto contract). Values match
-- internal/domain/review.go.
CREATE TABLE reviews (
    review_id      UUID PRIMARY KEY,
    application_id UUID NOT NULL CONSTRAINT reviews_application_id_key UNIQUE
                       REFERENCES application_participants (application_id),
    employer_id    UUID NOT NULL,
    score          SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    decision       TEXT NOT NULL CHECK (decision IN ('SHORTLISTED', 'REJECTED')),
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE reviews;
DROP TABLE application_participants;
