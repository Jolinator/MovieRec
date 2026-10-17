# Local Compose secrets

The files in this directory are templates for the ignored local <code>secrets/</code> directory. They are for local development only; GCP deployment examples live under <code>deploy/gcp/secrets-examples/</code>.

## Generate local files

Run this from the repository root:

~~~bash
bash scripts/init-local-secrets.sh
~~~

The script copies missing env templates, fills placeholder database passwords, and generates local TLS and JWT key pairs. Existing settings and complete key pairs are preserved. Incomplete key pairs stop the script so you can restore the missing file instead of silently replacing a key.

Set <code>TMDB_READ_ACCESS_TOKEN</code> in <code>secrets/tmdb.env</code> manually. The token must be obtained from TMDB; the script leaves its placeholder unchanged.

Start the Compose stack from the repository root with <code>docker compose up --build -d</code>. The first start downloads the model files. <code>docker compose down</code> keeps named volumes; <code>docker compose down -v</code> removes local database, model, and monitoring data.
