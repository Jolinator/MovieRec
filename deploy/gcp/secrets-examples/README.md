# GCP configuration and secret examples

These tracked examples are the source for GCP Make and Terraform settings. Do not put real credentials in this directory.

## Configuration discovery

<code>make secrets-init</code> starts with <code>make glean-vars</code>. Gleaning:

1. Reads the project from the active <code>gcloud</code> project.
2. Uses the existing app VM's zone when a VM named from <code>MOVIEREC_RESOURCE_NAME</code> is found.
3. Reads the model VM's zone, machine type, and attached GPU when that VM exists. GPU type, count, and machine type are saved for Make and Terraform.
4. For a fresh deployment, uses the configured Compute zone or <code>GCP_ZONE</code> in <code>server.env.example</code>; the example's CPU/GPU defaults remain in effect until a model VM exists.
5. Derives <code>GCP_REGION</code> from the app zone and writes the discovered settings to <code>deploy/gcp/secrets/server.env</code> and <code>terraform.tfvars</code>.

Make defaults live in <code>server.env.example</code>. Terraform values live in <code>deploy/gcp/terraform/terraform.tfvars.example</code>. The project, region, zone, and resource name are written by gleaning; Terraform's other settings are copied from the example. Existing tfvars values are retained. Choose a fresh-deployment zone with <code>gcloud config set compute/zone ZONE</code> before gleaning.

## Generated deployment secrets

<code>make secrets-init</code> copies missing service env files from these examples, fills placeholder database password pairs, and generates PostgreSQL TLS and JWT key files. It preserves matching passwords and existing key pairs. It does not create the TMDB access token; set <code>TMDB_READ_ACCESS_TOKEN</code> in <code>deploy/gcp/secrets/tmdb.env</code> before <code>make k8s-config</code>.

For an existing database, restore its original <code>deploy/gcp/secrets/</code> before running <code>make secrets-init</code>. If example password placeholders are used against an existing database, the generator will create new credentials that do not match the database.

The ignored <code>deploy/gcp/secrets/</code> directory contains the local service env files, <code>jwt-private.pem</code>, <code>jwt-public.pem</code>, and <code>tls/server.key</code>/<code>tls/server.crt</code>. Back these files up securely. Replacing the JWT pair invalidates existing login tokens. The separate public HTTPS certificate is generated and stored as a Kubernetes Secret by <code>make k8s-config</code>.
