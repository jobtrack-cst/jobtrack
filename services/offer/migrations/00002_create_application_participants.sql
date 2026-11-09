-- +goose Up
CREATE TABLE application_participants (
    application_id UUID PRIMARY KEY,
    applicant_id UUID NOT NULL,
    employer_id UUID NOT NULL
);

-- +goose Down
DROP TABLE application_participants;
