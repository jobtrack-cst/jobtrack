-- +goose Up
CREATE TABLE job_alerts (
 alert_id uuid PRIMARY KEY,
 applicant_id uuid NOT NULL,
 category_ids text[] NOT NULL CHECK (cardinality(category_ids)<=20),
 tag_ids text[] NOT NULL CHECK (cardinality(tag_ids)<=20),
 location varchar(100) NOT NULL,
 names jsonb NOT NULL,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 CHECK (cardinality(category_ids)>0 OR cardinality(tag_ids)>0 OR location<>''),
 UNIQUE(applicant_id, category_ids, tag_ids, location)
);
CREATE INDEX job_alerts_applicant_order ON job_alerts(applicant_id,created_at,alert_id);
-- +goose Down
DROP TABLE job_alerts;
