# GCP deployment kickstart

This guide takes a fresh clone to a running MovieRec deployment from Google Cloud Shell. It also covers how to continue an existing deployment safely, configure optional GPU generation, and use every Make target.

## Before you start

You need an authenticated Google Cloud account, an active project with billing enabled, and permission to create Compute Engine, networking, Artifact Registry, and service-account resources. Run the commands below from the repository root.

~~~bash
gcloud config set project PROJECT_ID
make cloud-shell-setup
~~~

The project ID is read from the active gcloud configuration by <code>make glean-vars</code>. You do not need to enter it in an environment file. For a fresh deployment, the example zone is <code>europe-west4-b</code>; choose another zone first if needed:

~~~bash
gcloud config set compute/zone ZONE
~~~

## Existing deployment or fresh deployment?

For an **existing deployment**, use the checkout that already manages it if possible. Terraform state is local to the checkout. In a new Cloud Shell or clone, restore the existing <code>deploy/gcp/terraform/terraform.tfstate</code>, <code>deploy/gcp/terraform/terraform.tfvars</code>, and <code>deploy/gcp/secrets/</code> before planning or applying. The state and tfvars keep Terraform attached to the existing resources and settings; the secret files preserve database passwords, JWT keys, and certificates.

Gleaning can discover the active project and the app VM's zone, but it cannot recover Terraform state or credentials. Restore the old secret files before <code>make secrets-init</code>; its password step fills placeholders, while preserving matching passwords that are already set.

For a **fresh deployment**, no state or secret files need restoring. Gleaning uses the active project and configured Compute zone, or the example zone when no zone is configured.

## Fresh deployment kickstart

1. Prepare configuration and secret files:

   ~~~bash
   make secrets-init
   ~~~

   This runs <code>make glean-vars</code>, writes the active project and VM settings to <code>server.env</code> and <code>terraform.tfvars</code>, copies missing secret examples, creates local TLS/JWT keys, and fills placeholder database passwords. If a model VM exists, it also records its zone and GPU configuration. It does not generate a TMDB token. Edit <code>deploy/gcp/secrets/tmdb.env</code> and set <code>TMDB_READ_ACCESS_TOKEN</code> before Kubernetes configuration.

2. Review and provision the infrastructure:

   ~~~bash
   make infra-plan
   make infra-apply
   ~~~

   Review the full plan before applying. On a fresh deployment the example config provisions CPU model hardware by default. Terraform creates the network, firewall rules, VMs, disks, NAT, public IP, and Artifact Registry. Running VMs incur charges; retained disks, IPs, and registry data can continue to incur charges after <code>make stop</code>.

3. Join the nodes and configure the cluster:

   ~~~bash
   make cluster-bootstrap
   make k8s-config
   make models
   ~~~

   <code>cluster-bootstrap</code> joins the database and model VMs to K3s. GPU runtime and Kubernetes device-plugin setup are included automatically if the model VM has a GPU. <code>k8s-config</code> applies ConfigMaps, Secrets, and the HTTPS certificate; on first setup, enter a Grafana admin password when prompted. <code>models</code> downloads and verifies the model files on the model VM.

4. Build and deploy the app:

   ~~~bash
   make update-app IMAGE_TAG=v1
   ~~~

   Use a new image tag for each release. Get the public IP and check health with:

   ~~~bash
   terraform -chdir=deploy/gcp/terraform output -raw public_ip
   curl -k https://PUBLIC_IP.nip.io/movieAPI/health
   ~~~

   The default hostname uses the IP-based nip.io name and a self-signed certificate, so browsers display a certificate warning. To use a custom hostname, point DNS at the public IP and pass the same <code>PUBLIC_HOST</code> to both <code>make k8s-config</code> and <code>make update-app</code>.

## GPU options

CPU is the fresh-deployment default. GPU hardware is attached only to the model VM. The Terraform setting is saved in <code>deploy/gcp/terraform/terraform.tfvars</code>; the Make variable chooses whether to change or follow that setting.

On an existing deployment, <code>make glean-vars</code> reads the model VM's machine type and accelerator details, then saves its zone, GPU type/count, and machine type. If the model VM is absent, it keeps the configured example/local defaults.

| Setting | Meaning |
|---|---|
| <code>GPU=auto</code> | Keep the saved Terraform GPU choice. This is the normal value for apply, bootstrap, and deploy. |
| <code>GPU=on</code> | Save GPU enabled in Terraform. Use it on <code>make infra-plan</code> when enabling a GPU. |
| <code>GPU=off</code> | Save GPU disabled in Terraform, or select CPU manifests on deployment. |
| <code>GPU_TYPE</code> | Accelerator model. Supported values are listed below. |
| <code>GPU_MACHINE_TYPE</code> | Optional compatible machine-family override; empty uses Terraform's recommended type. |
| <code>GPU_COUNT</code> | For N1-attached T4 GPUs, choose 1, 2, or 4. Other accelerator-optimized machines determine their GPU count from the machine type. |
| <code>GPU_REGION</code> | <code>europe</code> or <code>us</code>, for the capacity trial command only. |

Supported <code>GPU_TYPE</code> values:

- T4: <code>nvidia-tesla-t4</code>, <code>nvidia-tesla-t4-vws</code>
- L4: <code>nvidia-l4</code>, <code>nvidia-l4-vws</code>
- RTX PRO 6000: <code>nvidia-rtx-pro-6000</code>, <code>nvidia-rtx-pro-6000-vws</code>
- Datacenter GPUs: <code>nvidia-h100-80gb</code>, <code>nvidia-h100-mega-80gb</code>, <code>nvidia-h200-141gb</code>, <code>nvidia-b200</code>

Enable a GPU by selecting it during the plan. Then apply and bootstrap using <code>auto</code> so those commands keep the saved choice:

~~~bash
make infra-plan GPU=on GPU_TYPE=nvidia-tesla-t4
make infra-apply GPU=auto
make cluster-bootstrap GPU=auto
make update-app IMAGE_TAG=v2 GPU=auto
~~~

The selected accelerator must be available in the model VM's zone and quota. GPU changes affect only the model VM, can replace it, and add GPU charges while it runs. If T4/L4 capacity is unavailable, <code>make gpu-capacity GPU_REGION=europe</code> or <code>GPU_REGION=us</code> offers model-only placement trials across documented zones. It prompts before applying and saves the successful model zone.

To switch back to CPU, deploy the CPU manifests first, then change the Terraform hardware:

~~~bash
make update-app IMAGE_TAG=v3 GPU=off
make infra-plan GPU=off
make infra-apply GPU=auto
make cluster-bootstrap GPU=auto
~~~

## Make target reference

All Make defaults are maintained in <code>deploy/gcp/secrets-examples/server.env.example</code> and <code>deploy/gcp/terraform/terraform.tfvars.example</code>. The ignored local files override those examples. Required values such as the image tag and migration zones are supplied on the command line.

| Target | What it does |
|---|---|
| <code>help</code> | Prints the command summary and GPU usage hints. |
| <code>cloud-shell-setup</code> | Installs Micro, Terraform, and Ansible in Cloud Shell, applies the configured Docker upload limit, and enables optional IAP upload acceleration. |
| <code>glean-vars</code> | Reads the active GCP project, discovers existing VM zones and model GPU settings, derives the app region, and saves settings for Make and Terraform. |
| <code>secrets-init</code> | Runs gleaning, creates missing secret files and TLS/JWT keys, then fills placeholder database passwords. Restore old secrets first for an existing database. |
| <code>secrets-passwords</code> | Manually fills placeholder database passwords; matching configured values are preserved. Usually run automatically by <code>secrets-init</code>. |
| <code>infra-plan</code> | Initializes, formats, validates, and plans Terraform changes. <code>GPU=on</code> or <code>off</code> updates the saved GPU setting. |
| <code>infra-apply</code> | Initializes, formats, validates, and applies Terraform changes. Review the plan first; <code>TF_AUTO_APPROVE=1</code> skips Terraform's confirmation prompt. |
| <code>cluster-bootstrap</code> | Checks app SSH access, joins the worker VMs to K3s, and installs GPU runtime/device-plugin support when the model VM has a GPU. |
| <code>k8s-config</code> | Creates or updates cluster ConfigMaps and Secrets, Grafana credentials, and the public HTTPS certificate. |
| <code>models</code> | Runs the model download/verification job and waits for it to finish. |
| <code>build-push</code> | Builds backend and frontend images and pushes them to Artifact Registry. Requires <code>IMAGE_TAG</code>. |
| <code>deploy</code> | Applies the Kubernetes manifests for an existing image tag and waits for workload rollouts. Requires <code>IMAGE_TAG</code>. |
| <code>update-app</code> | Runs <code>build-push</code> followed by <code>deploy</code> for one tag. Requires <code>IMAGE_TAG</code>. |
| <code>set-region</code> | Migrates app/database VMs, or all VMs, to another zone using disk snapshots; then reconnects, builds/deploys, and cleans up old registry copies. Requires <code>OLD_ZONE</code> and <code>TARGET_ZONE</code>; asks for confirmation before migration. |
| <code>artifact-registry-cleanup</code> | Lists Artifact Registry repositories outside the keep location and asks before deleting them. It applies to all repositories in the project, not only MovieRec. |
| <code>gpu-capacity</code> | Prompts, then tries model-only T4/L4 Terraform applies in documented zones for <code>GPU_REGION=europe</code> or <code>us</code>. |
| <code>ssh-machine</code> | Opens an interactive IAP SSH session to <code>MACHINE=app</code>, <code>database</code>, or <code>model</code>. |
| <code>grafana</code> | Opens a temporary IAP tunnel to Grafana in a local browser. |
| <code>container-logs</code> | Streams logs from a workload or finds its workload by <code>CONTAINER</code>; accepts <code>NAMESPACE</code>, <code>TAIL_LINES</code>, and <code>FOLLOW=1</code>. |
| <code>pause-app</code> | Scales frontend/API/model workloads to zero while leaving databases and monitoring running. |
| <code>resume-app</code> | Restores the frontend/API/model workloads to one replica. |
| <code>reboot</code> | Reboots one VM or all three in sequence. Set <code>MACHINE</code>. |
| <code>clear-databases</code> | Deletes the three PostgreSQL data claims and starts empty databases. Requires <code>CONFIRM_CLEAR_DATABASES=1</code>; this erases database data. |
| <code>clear-databases-reboot-all</code> | Clears the databases, then reboots every VM. Requires <code>CONFIRM_CLEAR_DATABASES=1</code>. |
| <code>start</code> | Starts the existing VMs through gcloud without requiring Terraform state. |
| <code>stop</code> | Stops the existing VMs; disks, IP addresses, and registry resources remain. |
| <code>destroy</code> | Plans and destroys Terraform-managed resources, including attached data disks. Requires <code>CONFIRM_DESTROY=1</code>. |
| <code>docker-push-limit</code> | Sets the Docker concurrent upload limit from the example/local Make settings; requires sudo. |
| <code>docker-push-limit-reset</code> | Removes that Docker upload limit and restores Docker's default; requires sudo. |
| <code>benchmark</code> | Runs ApacheBench against a ready API endpoint. Configure <code>AB_URL</code>, request count, and concurrency with environment variables. |
| <code>benchmark-stress</code> | Runs the health, movie-list, large-page, and genre-load profiles. Configure the <code>AB_*</code> profile variables. |

### Common options

- <code>IMAGE_TAG</code> is required by image build/deploy targets; use a new tag per release.
- <code>PUBLIC_HOST</code> is optional; when set, pass it to both <code>k8s-config</code> and <code>update-app</code>.
- <code>MODEL_ONLY=1</code> limits Terraform plan/apply to the model VM and related K3s firewall rules.
- <code>TF_PARALLELISM</code> sets Terraform parallelism; <code>TF_AUTO_APPROVE=1</code> is intended for reviewed automation.
- <code>SCOPE=app-db</code> moves app/database VMs; <code>SCOPE=all</code> also moves the model VM.
- <code>KEEP_LOCATION</code> sets the Artifact Registry location to retain; <code>artifact-registry-cleanup</code> deletes every other repository location in the project.
- <code>DOCKER_MAX_CONCURRENT_UPLOADS</code> sets the Docker push limit. <code>NAMESPACE</code>, <code>TAIL_LINES</code>, and <code>FOLLOW</code> configure <code>container-logs</code>.
- <code>make stop</code> retains infrastructure; <code>make destroy</code> deletes Terraform-managed disks and resources.
