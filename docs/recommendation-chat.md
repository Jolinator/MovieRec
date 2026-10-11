# Recommendation Chat setup

The recommendation chat uses two llama.cpp servers:

- `llama-embedding` loads an English BGE-small GGUF model and creates 384-dimensional synopsis/query embeddings.
- `llama-generation` loads a small Qwen instruct GGUF model and writes the response shown by the chat.

The `model-downloader` Compose service downloads the pinned GGUF files from
Hugging Face, verifies their SHA-256 checksums, and stores them in the
persistent `model-data` volume. The llama.cpp services start only after the
download succeeds. Subsequent starts reuse the verified files.

The backend generates missing movie synopsis embeddings after the movies database and inference services become available. During this process, movie browsing, login, and signup remain available. The chat endpoint returns `503 Service Unavailable` with a message asking the user to wait until the recommendation service is ready.

## Background recommendations

After login, the Home page requests `GET /movieAPI/recommendations`. The
backend generates up to three recommendations from the average embedding of
the movies in the user's My List, excluding movies already in that list. The
recommendations are stored in `recommendations-postgres`; movie details remain
in `movies-postgres`.

The cache is marked for refresh during logout and is refreshed asynchronously.
Login also requests a refresh when no cache exists or when the previous logout
refresh was not completed. The Home page does not wait for generation: it
shows a preparing message while the cache is pending, and explains that My
List must contain movies when the cache is empty. A ready cache remains stable
for the current login session.

The movies database uses `pgvector`. When setting up an existing checkout,
the new recommendations migration must be applied to the existing
`recommendations-postgres` volume. The simplest clean setup is to recreate all
database volumes once. This deletes the local authentication and movie data
and causes the movies to be imported again:

```bash
docker compose down --volumes
docker compose up --build
```

If the existing authentication and movie data must be preserved, apply
`database/migrations/recommendations/002_create_user_recommendations.sql`
once with `psql` against the recommendations database instead of deleting all
volumes.

To force the model files to be downloaded again, remove the model volume:

```bash
docker compose down
docker volume rm g21_model-data
docker compose up --build
```

Replace `g21_model-data` with the volume name shown by `docker volume ls` if
the Compose project has a different name.
