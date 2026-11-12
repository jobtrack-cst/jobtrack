-- +goose Up
-- JT-56: the latest verification decision per employer, from the
-- employer.verified and employer.verification_rejected events. Kept apart
-- from company_profiles so a decision that arrives before the profile is not
-- lost; Create copies it into the new profile.
CREATE TABLE employer_verifications (
    employer_id      UUID PRIMARY KEY,
    status           TEXT NOT NULL CHECK (status IN ('verified', 'rejected')),
    -- Same limit as the company-verification reject reason (JT-33).
    rejection_reason TEXT NOT NULL DEFAULT '' CHECK (char_length(rejection_reason) <= 500),
    -- The event occurred_at; only a later decision replaces this row.
    decided_at       TIMESTAMPTZ NOT NULL,
    event_id         UUID NOT NULL,
    CHECK ((status = 'verified' AND rejection_reason = '')
        OR (status = 'rejected' AND rejection_reason <> ''))
);

-- Event IDs the consumer has handled. ApplyVerification writes this row and
-- the decision in one transaction, so a failed write never marks an event as
-- done (same pattern as audit-log and analytics).
CREATE TABLE processed_events (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE employer_verifications;
