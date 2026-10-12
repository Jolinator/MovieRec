CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE movie_embeddings (
    tmdb_id INTEGER PRIMARY KEY,
    embedding vector(384) NOT NULL
);

CREATE INDEX movie_embeddings_embedding_idx
    ON movie_embeddings USING hnsw (embedding vector_cosine_ops);
