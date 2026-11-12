-- +goose Up
-- Limits match internal/domain/profile.go (JT-50). VARCHAR(n) counts
-- characters. Year upper bounds move with the clock, so only the domain
-- checks them.
CREATE TABLE applicant_profiles (
    -- The applicant's user ID from auth; one profile per applicant.
    applicant_id UUID PRIMARY KEY,
    summary      VARCHAR(2000) NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL
);

-- Education and skills keep the order they were sent in (position).
-- UpdateMyProfile replaces both lists, so rows are deleted and reinserted.
CREATE TABLE profile_education (
    applicant_id  UUID NOT NULL REFERENCES applicant_profiles (applicant_id) ON DELETE CASCADE,
    position      SMALLINT NOT NULL CHECK (position BETWEEN 0 AND 19),
    institution   VARCHAR(150) NOT NULL CHECK (institution <> ''),
    qualification VARCHAR(150) NOT NULL CHECK (qualification <> ''),
    start_year    INTEGER NOT NULL CHECK (start_year >= 1950),
    -- 0 means ongoing.
    end_year      INTEGER NOT NULL CHECK (end_year = 0 OR end_year >= start_year),
    PRIMARY KEY (applicant_id, position)
);

CREATE TABLE profile_skills (
    applicant_id UUID NOT NULL REFERENCES applicant_profiles (applicant_id) ON DELETE CASCADE,
    position     SMALLINT NOT NULL CHECK (position BETWEEN 0 AND 49),
    skill        VARCHAR(50) NOT NULL CHECK (skill <> ''),
    PRIMARY KEY (applicant_id, position)
);

-- Duplicate skills are compared ignoring case, as in the domain.
CREATE UNIQUE INDEX profile_skills_unique_idx ON profile_skills (applicant_id, lower(skill));

-- +goose Down
DROP TABLE profile_skills;
DROP TABLE profile_education;
DROP TABLE applicant_profiles;
