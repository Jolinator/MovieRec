# GitLab CI/CD

The pipeline validates and tests changes, then builds images. A separate manual
job can deploy the app from `master`:

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
- `docker_images` builds one image per backend service (`gateway`, `auth`,
  `movies`, `recommendations`, all from `backend/Dockerfile` with a `SERVICE`
  build argument) and the frontend image, tagged with the commit SHA.
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

The `docker_images` job publishes no images; it builds them in GitLab CI. The
manual GCP deployment job builds and pushes the same services to Google Artifact
Registry as part of `make update-app`.

## Deploying the app to GCP

The `update_app` job is a manual deployment from `master`. After the validation,
test, and image-build stages pass, it runs
`make update-app IMAGE_TAG=$CI_COMMIT_SHA`. It builds and pushes the images to
Google Artifact Registry, then uses Ansible over IAP SSH to deploy them and wait
for rollouts. It does not run Terraform apply, configure Kubernetes secrets, or
download model files.

Before using the job:

1. Enable Terraform/OpenTofu state storage for this project on the self-managed
   GitLab instance. The CI state is stored as `movierec` in the project's GitLab
   Terraform state backend. CI uses its job token to read the Terraform outputs.
2. Migrate the existing local Terraform state from a trusted machine. Without
   the state, `make update-app` cannot find the Artifact Registry, VM names, or
   IAP connection details. From `deploy/gcp/terraform`, set the HTTP backend
   variables below using the project's numeric GitLab ID and a GitLab access
   token with API scope, then run `terraform init -migrate-state` and confirm
   the copy. Preserve a secure backup of the local state until you have checked
   Terraform outputs from the remote state.

   ```sh
   export TF_HTTP_ADDRESS="https://gitlab.rnl.tecnico.ulisboa.pt/api/v4/projects/<PROJECT_ID>/terraform/state/movierec"
   export TF_HTTP_LOCK_ADDRESS="$TF_HTTP_ADDRESS/lock"
   export TF_HTTP_UNLOCK_ADDRESS="$TF_HTTP_ADDRESS/lock"
   export TF_HTTP_USERNAME="<GITLAB_USERNAME>"
   export TF_HTTP_PASSWORD="<GITLAB_API_TOKEN>"
   export TF_HTTP_LOCK_METHOD=POST
   export TF_HTTP_UNLOCK_METHOD=DELETE
   terraform init -migrate-state
   ```

3. Configure protected GitLab CI/CD variables for the `master` branch:
   - `TF_VAR_project_id`: the Google Cloud project ID.
   - `GCP_SERVICE_ACCOUNT_KEY`: a **file** variable containing a JSON key for a
     dedicated service account that can push to Artifact Registry and connect to
     the app VM through IAP. Keep the variable protected and limit who can
     change protected variables. This job currently uses a service-account key;
     Workload Identity Federation can replace it in a later hardening step.
   - `PUBLIC_HOST`, if the deployment should use a DNS name instead of the
     default IP based `nip.io` hostname.
   - `MOVIEREC_SSH_USER`, if Ansible should connect as a specific SSH user.
4. Protect the `production` environment in GitLab so only intended maintainers
   can start the manual deployment. Keep the project private because Terraform
   state and CI variables can contain sensitive information.

The local Makefile Terraform targets still work with the HTTP backend. Use the
same GitLab state backend credentials locally once migration is complete; avoid
working against an old local state copy.

The `backend_tests` job requires a masked GitLab CI/CD variable named
`TMDB_READ_ACCESS_TOKEN`. It is used only to import the test movie catalogue;
the CI job creates the remaining local test configuration automatically.
