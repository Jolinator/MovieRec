CREATE TABLE user_recommendations (
    user_id UUID PRIMARY KEY,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    generated_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'ready', 'empty', 'failed'))
);

CREATE TABLE user_recommendation_items (
    user_id UUID NOT NULL REFERENCES user_recommendations(user_id) ON DELETE CASCADE,
    tmdb_id INTEGER NOT NULL,
    position INTEGER NOT NULL CHECK (position BETWEEN 1 AND 9),
    PRIMARY KEY (user_id, position),
    UNIQUE (user_id, tmdb_id)
);
