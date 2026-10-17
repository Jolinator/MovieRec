# CI/CD

The GitLab pipeline validates and tests changes, builds container images, and provides a manual production deploy. Pipeline rules include merge requests targeting <code>master</code> or <code>CI/CD_testing</code>, and commits on those branches. The production deploy is available only as a manual job on <code>master</code>.

## Jobs

| Stage | Job | Work |
|---|---|---|
| Validate | <code>backend_validation</code> | Checks Go formatting and runs <code>go vet ./...</code>. |
| Validate | <code>frontend_validation</code> | Installs locked npm dependencies and creates the production build. |
| Validate | <code>docker_images</code> | Builds the four Go service images and the frontend image with Docker-in-Docker; does not publish them. |
| Test | <code>frontend_tests</code> | Runs the Vitest suite. |
| Test | <code>backend_unit_tests</code> | Runs Go unit tests and saves <code>coverage.out</code> as an artifact. |
| Test | <code>backend_tests</code> | Runs the backend integration scenarios against temporary Compose services. |
| Test | <code>apachebench_stress</code> | Starts a temporary stack and runs the health, movie-list, large-page, and genre load profiles. |
| Deploy | <code>update_app</code> | Manual deployment from <code>master</code>; builds/pushes the commit-tagged images and deploys them. |

The Docker-in-Docker test jobs need a GitLab runner configured for privileged Docker services. Integration and load-test jobs require a masked CI/CD variable named <code>TMDB_READ_ACCESS_TOKEN</code>. The setup script creates temporary test configuration and does not use the production secret directory.

## Manual GCP deployment

The <code>update_app</code> job assumes the GCP VMs, K3s cluster, Kubernetes Secrets, and model files already exist. It does not run Terraform, <code>make secrets-init</code>, <code>make k8s-config</code>, or <code>make models</code>. The image tag is the commit SHA.

Configure protected GitLab variables:

- <code>TF_VAR_project_id</code>: project ID used to authenticate gcloud.
- <code>GCP_SERVICE_ACCOUNT_KEY</code>: file variable with a service-account JSON key. Grant Artifact Registry push, Compute Engine instance list/read, and IAP SSH permissions.
- Optional <code>GCP_ARTIFACT_REGISTRY_REGION</code>: set only if the registry is in a different region.
- Optional <code>MOVIEREC_RESOURCE_NAME</code>, <code>MOVIEREC_SSH_USER</code>, and <code>PUBLIC_HOST</code>: override the job defaults or use a custom hostname.

Protect the <code>production</code> environment and the deployment variables. After authenticating, the job runs <code>make glean-vars</code> to discover the app VM zone and derive its region. No <code>GCP_ZONE</code> or <code>GCP_REGION</code> CI variable is needed. The optional registry-region override is retained when set; otherwise image publishing uses the discovered region.

After a region migration, the next deploy discovers the new app VM zone automatically. Update the registry-region override only if the registry is in a different region. The <code>make set-region</code> workflow handles infrastructure migration, image deployment, and old MovieRec registry copies from Cloud Shell; the CI job itself does not manage Terraform state.

## Performance commands

For a running local gateway, <code>make benchmark</code> runs ApacheBench against the health endpoint by default. Override <code>AB_URL</code>, <code>AB_REQUESTS</code>, and <code>AB_CONCURRENCY</code> as needed. <code>make benchmark-stress</code> runs four read-only request profiles. The scripts accept <code>BACKEND_HOST</code>/<code>AB_HOST</code>, <code>BACKEND_PORT</code>, and the per-profile <code>AB_*</code> settings.
