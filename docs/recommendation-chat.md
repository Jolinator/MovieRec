# Recommendation Chat setup

The recommendation chat uses two llama.cpp servers:

- `llama-embedding` loads an English BGE-small GGUF model and creates 384-dimensional synopsis/query embeddings.
- `llama-generation` loads a small Qwen instruct GGUF model and writes the response shown by the chat.

The `model-downloader` Compose service downloads the pinned GGUF files from
Hugging Face, verifies their SHA-256 checksums, and stores them in the
persistent `model-data` volume. The llama.cpp services start only after the
download succeeds. Subsequent starts reuse the verified files.

The backend generates missing movie synopsis embeddings after the movies database and inference services become available. During this process, movie browsing, login, and signup remain available. The chat endpoint returns `503 Service Unavailable` with a message asking the user to wait until the recommendation service is ready.

The movies database uses `pgvector`. When setting up an existing checkout,
recreate the database volumes once so the migrations are applied. This deletes
the local authentication and movie data and causes the movies to be imported
again:

```bash
docker compose down --volumes
docker compose up --build
```

To force the model files to be downloaded again, remove the model volume:

```bash
docker compose down
docker volume rm g21_model-data
docker compose up --build
```

Replace `g21_model-data` with the volume name shown by `docker volume ls` if
the Compose project has a different name.
