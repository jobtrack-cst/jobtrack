-- +goose Up
CREATE TABLE offers (
    offer_id UUID PRIMARY KEY,
    application_id UUID NOT NULL CONSTRAINT offers_application_id_key UNIQUE,
    employer_id UUID NOT NULL,
    applicant_id UUID NOT NULL,
    terms VARCHAR(2000) NOT NULL CHECK (char_length(terms) >= 1),
    status TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'declined')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE offers;
