-- +goose Up
-- Lengths match the domain limits in internal/domain/request.go. VARCHAR(n)
-- counts characters, like the domain.
CREATE TABLE verification_requests (
    id                  UUID PRIMARY KEY,
    -- The employer's user ID from auth.
    employer_id         UUID NOT NULL,
    registration_number VARCHAR(50) NOT NULL CHECK (registration_number <> ''),
    notes               VARCHAR(1000) NOT NULL DEFAULT '',
    status              TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    rejection_reason    VARCHAR(500) NOT NULL DEFAULT '',
    -- The admin who decided. NULL while pending.
    reviewed_by         UUID,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT verification_requests_reviewer_when_decided
        CHECK ((status = 'pending') = (reviewed_by IS NULL)),
    CONSTRAINT verification_requests_reason_only_when_rejected
        CHECK ((status = 'rejected') = (rejection_reason <> ''))
);

-- At most one PENDING and one APPROVED request per employer, even when two
-- submits or decisions race.
CREATE UNIQUE INDEX verification_requests_one_pending
    ON verification_requests (employer_id) WHERE status = 'pending';
CREATE UNIQUE INDEX verification_requests_one_approved
    ON verification_requests (employer_id) WHERE status = 'approved';

-- ListMyVerificationRequests (newest first) and ListVerificationRequests
-- (oldest first, optionally by status).
CREATE INDEX verification_requests_employer_created
    ON verification_requests (employer_id, created_at DESC, id DESC);
CREATE INDEX verification_requests_status_created
    ON verification_requests (status, created_at, id);

-- +goose Down
DROP TABLE verification_requests;
