CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE movie_lists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE movie_list_items (
    list_id UUID NOT NULL REFERENCES movie_lists(id) ON DELETE CASCADE,
    tmdb_id INTEGER NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (list_id, tmdb_id)
);

CREATE INDEX movie_list_items_tmdb_id_idx ON movie_list_items (tmdb_id);
