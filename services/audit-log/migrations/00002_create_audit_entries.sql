-- +goose Up
-- Lengths match the domain limits in internal/domain/entry.go. event_type and
-- producer are ASCII by their domain patterns, so VARCHAR(n) characters equal
-- bytes. There is no actor column until EventEnvelope has actor_id
-- (AGENTS.md Section 16); adding it later is a new nullable column.
CREATE TABLE audit_entries (
    -- The envelope event_id.
    event_id    UUID PRIMARY KEY,
    event_type  VARCHAR(100) NOT NULL CHECK (event_type <> ''),
    producer    VARCHAR(50) NOT NULL CHECK (producer <> ''),
    occurred_at TIMESTAMPTZ NOT NULL,
    -- TEXT, not JSONB, so the admin view shows the data exactly as received.
    -- The smallest JSON object is "{}", two bytes.
    data_json   TEXT NOT NULL CHECK (octet_length(data_json) BETWEEN 2 AND 65536),
    -- When audit-log stored the entry. Internal only, never returned.
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ListAuditEntries, newest first, optionally by event type.
CREATE INDEX audit_entries_occurred ON audit_entries (occurred_at DESC, event_id DESC);
CREATE INDEX audit_entries_type_occurred ON audit_entries (event_type, occurred_at DESC, event_id DESC);

-- Event IDs the consumer has handled. RecordEntry writes this row and the
-- entry in one transaction, so a failed insert never marks an event as done.
CREATE TABLE processed_events (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE audit_entries;
