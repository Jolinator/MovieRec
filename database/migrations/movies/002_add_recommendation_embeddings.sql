CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE movies
    ADD COLUMN IF NOT EXISTS synopsis_embedding vector(384);

CREATE INDEX IF NOT EXISTS movies_synopsis_embedding_idx
    ON movies USING hnsw (synopsis_embedding vector_cosine_ops);
