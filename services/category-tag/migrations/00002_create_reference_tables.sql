-- +goose Up
-- JT-39: read-only reference lists. Rows come only from migrations; the
-- service has no write RPCs. Names: 1 to 100 characters, no leading or
-- trailing spaces, no control characters, unique ignoring case.
CREATE TABLE categories (
    category_id uuid PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100 AND name = btrim(name) AND name !~ '[[:cntrl:]]')
);
CREATE UNIQUE INDEX categories_name_lower_key ON categories (lower(name));

CREATE TABLE tags (
    tag_id uuid PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100 AND name = btrim(name) AND name !~ '[[:cntrl:]]')
);
CREATE UNIQUE INDEX tags_name_lower_key ON tags (lower(name));

-- Jobs store a location name in their location field (job-posting limit
-- 100 characters), and job-alert matches it exactly.
CREATE TABLE locations (
    location_id uuid PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100 AND name = btrim(name) AND name !~ '[[:cntrl:]]')
);
CREATE UNIQUE INDEX locations_name_lower_key ON locations (lower(name));

-- +goose Down
DROP TABLE locations;
DROP TABLE tags;
DROP TABLE categories;
