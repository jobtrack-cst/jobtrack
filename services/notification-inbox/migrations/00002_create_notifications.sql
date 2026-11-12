-- +goose Up
CREATE TABLE notifications (
    notification_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_id    UUID NOT NULL,
    type            TEXT NOT NULL CHECK (char_length(type) BETWEEN 1 AND 50),
    title           TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 150),
    body            TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
    related_id      TEXT, -- can be empty, but usually a UUID string
    read            BOOLEAN NOT NULL DEFAULT FALSE,
    source_event_id UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Idempotency constraint: one notification per event per recipient
    UNIQUE (recipient_id, source_event_id)
);

-- Index for ListMyNotifications (newest first), filtering by unread
CREATE INDEX notifications_recipient_created_idx ON notifications (recipient_id, created_at DESC);
CREATE INDEX notifications_recipient_unread_idx ON notifications (recipient_id) WHERE read = FALSE;

-- +goose Down
DROP TABLE notifications;
