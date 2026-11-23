-- +goose Up
-- JT-114: the applicant's full name from auth, copied once at submission.
-- Applications submitted before JT-114 keep an empty name. char_length counts
-- characters, not bytes, matching the 200-character domain limit.
ALTER TABLE applications
    ADD COLUMN applicant_name text NOT NULL DEFAULT ''
    CONSTRAINT applications_applicant_name_length CHECK (char_length(applicant_name) BETWEEN 0 AND 200);

-- +goose Down
ALTER TABLE applications DROP COLUMN applicant_name;
