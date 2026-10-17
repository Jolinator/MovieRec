# MovieRec

MovieRec is a movie discovery app with accounts, personal watch lists, recommendations, and recommendation chat. The repository contains a React frontend, Go API services, PostgreSQL databases, and llama.cpp embedding and generation servers.

## How it fits together

- The browser uses the Go gateway; the gateway routes requests to auth, movies, and recommendations services.
- Each backend service has its own PostgreSQL database. The recommendations service also uses pgvector and the two llama.cpp model servers.
- Prometheus and Grafana provide metrics and dashboards.
- Docker Compose runs the stack locally. The GCP deployment uses Terraform for VMs/networking, K3s for workloads, Ansible for cluster configuration, and Kustomize for Kubernetes manifests.

## Run locally

Install Docker Compose v2, Python 3, and OpenSSL. From the repository root, generate the ignored local secret files:

~~~bash
bash scripts/init-local-secrets.sh
~~~

The script copies missing templates, fills database password placeholders, and generates local TLS/JWT keys. It keeps existing settings and complete key pairs. Set <code>TMDB_READ_ACCESS_TOKEN</code> in <code>secrets/tmdb.env</code> manually; it requires a token from TMDB.

See [the local secret guide](secrets-example/README.md) for details.

Start the application:

~~~bash
docker compose up --build -d
docker compose ps
~~~

Open the site at <http://localhost:5173>. The API health endpoint is <http://localhost:8080/movieAPI/health>; Grafana and Prometheus are on ports 3000 and 9090. The first start downloads the model files and may take several minutes.

Stop the containers with <code>docker compose down</code>. That keeps named-volume data. <code>docker compose down -v</code> also deletes local databases, downloaded models, and monitoring data.

## GCP deployment

For a fresh GCP setup or an existing VM deployment, follow [the deployment kickstart](deploy/README.md). It covers the project and zone discovery, secrets, infrastructure, GPU options, and every Make target.

## Development and CI

- [GitLab pipeline and deployment notes](ci-cd/README.md)
- [Backend unit-test notes](testing/backend/unit/README.md)
- [Backend load evaluation](testing/backend/evaluation/instructions.md)

The backend's Go tests run from <code>backend/</code> with <code>go test ./...</code>. Frontend tests use Vitest: run <code>npm ci</code> and <code>npm run test:run</code> from <code>frontend/</code>.
