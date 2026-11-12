-- +goose Up
ALTER TABLE outgoing_events
    ADD COLUMN claim_token UUID,
    ADD COLUMN lease_until TIMESTAMPTZ,
    ADD COLUMN next_attempt_at TIMESTAMPTZ,
    ADD CONSTRAINT outgoing_events_claim_pair
        CHECK ((claim_token IS NULL) = (lease_until IS NULL));

-- Existing pending index orders eligible candidates by created_at and event_id.
-- Expired leases and due retries are filtered using database time at claim time.

-- +goose Down
ALTER TABLE outgoing_events
    DROP CONSTRAINT outgoing_events_claim_pair,
    DROP COLUMN next_attempt_at,
    DROP COLUMN lease_until,
    DROP COLUMN claim_token;
