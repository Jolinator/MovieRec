# GitLab CI/CD

The pipeline currently performs validation and image builds only:

- `backend_validation` checks Go formatting and runs `go vet`.
- `frontend_validation` installs the locked npm dependencies and runs the production build.
- `backend_tests` runs the integration scenarios from
  `testing/backend/TestRunner.py`, which starts the required Compose services
  and writes a test report. The job uses Docker-in-Docker, installs the Python
  dependencies from `testing/backend/requirements.txt`, and prepares
  throwaway Compose secrets from `secrets-example/`.
- `docker_images` builds one image per backend service (`gateway`, `auth`,
  `movies`, `recommendations`, all from `backend/Dockerfile` with a `SERVICE`
  build argument) and the frontend image, tagged with the commit SHA.

The frontend does not currently have a unit-test framework. Its production build is
the appropriate CI check for the current small frontend and catches compilation,
module-resolution, and bundling errors. Frontend tests can be added if the UI gains
substantial stateful behavior or a regression-prone feature.

The image job only builds images. Publishing them to Google Artifact Registry and
deploying to GCP will be added after CI authentication and deployment credentials
are configured.

The `backend_tests` job requires a masked GitLab CI/CD variable named
`TMDB_READ_ACCESS_TOKEN`. It is used only to import the test movie catalogue;
the CI job creates the remaining local test configuration automatically.
