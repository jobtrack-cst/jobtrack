-- +goose Up
-- Fail without changing historical data if existing rejected rows violate policy.
ALTER TABLE resumes ADD CONSTRAINT resumes_rejection_reason
    CHECK (validation_status <> 'rejected' OR
        (char_length(rejection_reason) BETWEEN 1 AND 500 AND
         rejection_reason = btrim(rejection_reason)));

-- +goose Down
ALTER TABLE resumes DROP CONSTRAINT resumes_rejection_reason;
