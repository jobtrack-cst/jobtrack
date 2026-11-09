-- +goose Up
-- One row per counted event, so GetSummary can count any exact time range.
-- Only the fields the summary needs are stored: no job, employer or
-- applicant IDs. Limits match internal/domain/outcome.go.
CREATE TABLE outcome_events (
    -- The envelope event_id.
    event_id    UUID PRIMARY KEY,
    event_type  TEXT NOT NULL CHECK (event_type IN (
        'job.published', 'job.closed',
        'application.submitted', 'application.withdrawn', 'application.status_changed',
        'offer.made', 'offer.accepted', 'offer.declined'
    )),
    -- StatusChanged.new_status as sent in the event, trimmed by the domain.
    -- TEXT with a length CHECK, because VARCHAR(n) would silently cut
    -- trailing spaces instead of refusing the value.
    new_status  TEXT CHECK (char_length(new_status) BETWEEN 1 AND 32),
    occurred_at TIMESTAMPTZ NOT NULL,
    -- When analytics stored the event. Internal only, never returned.
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- new_status is set for status changes and only for them.
    CONSTRAINT outcome_events_status_only_for_status_changes
        CHECK ((event_type = 'application.status_changed') = (new_status IS NOT NULL))
);

-- The summary filters only by occurred_at and groups by type and status, so
-- the index covers the whole query.
CREATE INDEX outcome_events_occurred ON outcome_events (occurred_at) INCLUDE (event_type, new_status);

-- Event IDs the consumer has handled. RecordEvent writes this row and the
-- outcome event in one transaction, so a failed insert never marks an event
-- as done.
CREATE TABLE processed_events (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE outcome_events;
