# GitLab CI/CD

The pipeline currently performs validation and image builds only:

- `backend_validation` checks Go formatting and runs `go vet`.
- `frontend_validation` installs the locked npm dependencies and runs the production build.
- `backend_tests` runs the Go test suite.
- `docker_images` builds the backend and frontend images, tagged with the commit SHA.

The frontend does not currently have a unit-test framework. Its production build is
the appropriate CI check for the current small frontend and catches compilation,
module-resolution, and bundling errors. Frontend tests can be added if the UI gains
substantial stateful behavior or a regression-prone feature.

The image job only builds images. Publishing them to Google Artifact Registry and
deploying to GCP will be added after CI authentication and deployment credentials
are configured.
