# MovieRec

MovieRec is a movie discovery app with accounts, personal watch lists, recommendations, and recommendation chat. The repository contains a React frontend, Go API services, PostgreSQL databases, and llama.cpp embedding and generation servers.

## How it fits together

- The browser uses the Go gateway; the gateway routes requests to auth, movies, and recommendations services.
- Each backend service has its own PostgreSQL database. The recommendations service also uses pgvector and the two llama.cpp model servers.
- Prometheus and Grafana provide metrics and dashboards.
- Docker Compose runs the stack locally. The GCP deployment uses Terraform for VMs/networking, K3s for workloads, Ansible for cluster configuration, and Kustomize for Kubernetes manifests.

## Navigate the repository

Start at the root files when you need to understand or run the project:

- `README.md` — project overview, local startup, deployment entry points, and links to testing documentation.
- `Makefile` — the command reference for infrastructure, deployment, model management, logs, and benchmarks.
- `compose.yaml` and `compose.services.yaml` — local Docker Compose orchestration.
- `.gitlab-ci.yml` — GitLab validation, test, image-build, and deployment pipeline.

The main directories are organized by responsibility:

```text
backend/                 Go API services and shared backend packages
frontend/                React/Vite web application
database/                PostgreSQL migrations and database-related files
deploy/                  GCP, Terraform, Ansible, and Kubernetes deployment configuration
monitoring/              Prometheus configuration and Grafana resources
scripts/                 Local, CI, model, secret, and GCP helper scripts
testing/                 Backend unit, integration, and load-test resources
ci-cd/                   CI helper scripts and pipeline notes
secrets-example/         Templates and instructions for local development secrets
```

Use these paths as the usual entry points:

- To change the API, begin in `backend/cmd/` for service entry points and `backend/internal/` for implementation.
- To change the web interface, begin in `frontend/src/`; use `frontend/package.json` for available commands.
- To change database structure, inspect `database/migrations/` and the service code that applies those migrations.
- To change local services, networking, or ports, inspect `compose.yaml`, `compose.services.yaml`, and `secrets-example/`.
- To change cloud infrastructure, begin with `deploy/README.md`, then inspect `deploy/gcp/terraform/`.
- To change cluster configuration or rollout behavior, inspect `deploy/k8s/` and `deploy/ansible/`.
- To change CI behavior, inspect `.gitlab-ci.yml` and the scripts under `ci-cd/`.
- To add or update tests, use the matching area under `testing/`; backend unit tests are also colocated with packages under `backend/internal/`.

Keep real credentials and generated deployment state out of version control. The local `secrets/` and GCP `deploy/gcp/secrets/` directories are ignored; use the tracked example files as templates.

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
