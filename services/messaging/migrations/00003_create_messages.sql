-- +goose Up
CREATE TABLE messages (
 message_id UUID PRIMARY KEY,
 application_id UUID NOT NULL REFERENCES application_participants(application_id),
 sender_id UUID NOT NULL,
 recipient_id UUID NOT NULL,
 content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 2000),
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL CHECK (updated_at >= created_at)
);
CREATE INDEX messages_thread_order ON messages(application_id, created_at, message_id);
-- +goose Down
DROP TABLE messages;
