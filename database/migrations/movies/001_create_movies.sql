CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE movies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tmdb_id INTEGER NOT NULL,
    title VARCHAR(500) NOT NULL,
    average_rating NUMERIC(3, 1) NOT NULL DEFAULT 0,
    poster_path TEXT,
    synopsis TEXT NOT NULL DEFAULT '',
    backdrop_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT movies_tmdb_id_unique UNIQUE (tmdb_id),
    CONSTRAINT movies_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT movies_average_rating_valid CHECK (average_rating >= 0 AND average_rating <= 10)
);

CREATE INDEX movies_title_idx ON movies (title);
