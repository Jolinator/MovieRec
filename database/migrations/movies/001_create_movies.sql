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

CREATE TABLE genres (
    id INTEGER PRIMARY KEY,
    name VARCHAR(100) NOT NULL
);

CREATE TABLE movie_genres (
    movie_id UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES genres(id) ON DELETE CASCADE,
    PRIMARY KEY (movie_id, genre_id)
);

CREATE INDEX movie_genres_genre_id_idx ON movie_genres (genre_id);

INSERT INTO genres (id, name) VALUES
    (28, 'Action'),
    (12, 'Adventure'),
    (16, 'Animation'),
    (35, 'Comedy'),
    (80, 'Crime'),
    (99, 'Documentary'),
    (18, 'Drama'),
    (10751, 'Family'),
    (14, 'Fantasy'),
    (36, 'History'),
    (27, 'Horror'),
    (10402, 'Music'),
    (9648, 'Mystery'),
    (10749, 'Romance'),
    (878, 'Science Fiction'),
    (10770, 'TV Movie'),
    (53, 'Thriller'),
    (10752, 'War'),
    (37, 'Western');
