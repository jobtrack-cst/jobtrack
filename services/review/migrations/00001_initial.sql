-- +goose Up
-- Reference migration: record a version without creating business tables.
SELECT 1;

-- +goose Down
SELECT 1;
