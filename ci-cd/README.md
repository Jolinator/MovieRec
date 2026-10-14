# GitLab CI/CD

The pipeline runs source validation and Docker image builds in parallel, then
tests changes. A separate manual job can deploy the app from `master`:

- `backend_validation` checks Go formatting and runs `go vet`.
- `frontend_validation` installs the locked npm dependencies and runs the production build.
- `frontend_tests` installs the locked npm dependencies and runs the Vitest frontend
  tests without requiring backend services.
- `backend_tests` runs the integration scenarios from
  `testing/backend/DockerTests/TestRunner.py`, which starts the required
  Compose services and writes a test report. The job uses Docker-in-Docker,
  installs Python dependencies from
  `testing/backend/DockerTests/requirements.txt`, and prepares throwaway
  Compose secrets from `secrets-example/`.
- `apachebench_stress` is a separate Docker-in-Docker job that starts a fresh
  gateway, backend-service, and database stack, waits for it to become ready,
  then runs the ApacheBench stress profiles.
- `docker_images` runs alongside validation and builds images for the backend
  services (`gateway`, `auth`, `movies`, and `recommendations`) from
  `backend/Dockerfile` with a `SERVICE` build argument, plus the frontend image.
  All images are tagged with the commit SHA.
- `update_app` runs `make update-app` to rebuild those images for Google Artifact
  Registry and deploy them to the GCP cluster.

The frontend uses Vitest with React Testing Library. The tests cover API request
contracts, login behavior, movie-grid rendering, and application navigation.

## Running ApacheBench

ApacheBench is an opt-in performance smoke test for a running gateway. Start
the backend first, then run:

```sh
make benchmark
```

The default benchmark sends 1,000 requests to
`http://localhost:8080/movieAPI/health` with 10 concurrent clients. Override
the target and load without changing the script:

```sh
AB_URL=http://localhost:8080/movieAPI/movies \
AB_REQUESTS=5000 AB_CONCURRENCY=50 make benchmark
```

The command performs a `curl` readiness check before invoking `ab`. Install
ApacheBench from the `apache2-utils` package when it is not already available.
It waits up to five minutes by default; override that with `AB_READY_TIMEOUT`.

For a broader, read-only infrastructure load test, run:

```sh
make benchmark-stress
```

It runs the existing health and movie-list profiles plus a 100-movie page
profile and a high-concurrency genre-list profile. All profiles use
`BACKEND_HOST` (or `AB_HOST`) and `BACKEND_PORT` when supplied, so the CI job
targets its Docker gateway rather than the CI container itself. Every profile
can be tuned with `AB_HEALTH_*`, `AB_MOVIES_*`, `AB_MOVIES_LARGE_PAGE_*`, or
`AB_GENRES_*` request/concurrency variables.

In CI, the integration runner first cleans up its temporary Compose services.
The `backend_tests` wrapper then starts a fresh gateway, backend-service, and
database stack, and waits for its databases and movie import to be ready before
the stress profiles run.

The `docker_images` job publishes no images; it builds them in GitLab CI. The
manual GCP deployment job builds and pushes the same services to Google Artifact
Registry as part of `make update-app`.

## Deploying the app to GCP

The `update_app` job is a manual deployment from `master`. After the validation
and test stages pass, it runs
`make update-app IMAGE_TAG=$CI_COMMIT_SHA`. It builds and pushes the images to
Google Artifact Registry, then uses Ansible over IAP SSH to deploy them with the
normal CPU Kubernetes overlay and wait for rollouts. It does not use the CUDA
image or GPU overlay. It also does not run Terraform apply, configure Kubernetes
secrets, or download model files.

Before using the job:

1. Configure protected GitLab CI/CD variables for the `master` branch:
   - `TF_VAR_project_id`: the Google Cloud project ID.
   - `GCP_SERVICE_ACCOUNT_KEY`: a **file** variable containing a JSON key for a
     dedicated service account that can push to Artifact Registry, describe the
     three Compute Engine instances, and connect to the app VM through IAP. Keep
     the variable protected and limit who can change protected variables. This
     job currently uses a service-account key; Workload Identity Federation can
     replace it in a later hardening step.
   - `GCP_ZONE` is required. Set it to the app/database zone provisioned by
     Terraform. The deployment job derives `GCP_REGION` from this zone, so no
     separate region variable is needed.
   - `MOVIEREC_RESOURCE_NAME`, if it differs from `movierec`.
   - `PUBLIC_HOST`, if the deployment should use a DNS name instead of the
     default IP based `nip.io` hostname.
   - `MOVIEREC_SSH_USER`, if Ansible should connect as a specific SSH user.
2. Protect the `production` environment in GitLab so only intended maintainers
   can start the manual deployment.

The `update_app` job does not initialize Terraform or access Terraform state.
The Terraform state is local to the Cloud Shell checkout and is not available
to GitLab. The job derives the region from the configured app/database zone,
builds the Artifact Registry path, then uses `gcloud` to read the configured VM
instances for Ansible. GitLab-managed Terraform state is optional and is only
needed if you want Terraform commands in different environments to share state.
Manual Cloud Shell Terraform commands can continue using their local state.

The `backend_tests` and `apachebench_stress` jobs require a masked GitLab CI/CD
variable named `TMDB_READ_ACCESS_TOKEN`. It is used only to import the test
movie catalogue; the CI jobs create the remaining local test configuration
automatically.
