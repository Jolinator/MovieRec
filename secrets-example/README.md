# Local secret templates

These templates mirror the structure of the ignored local `secrets/` directory. Copy the `.env.example` files to `secrets/` and replace every placeholder with local values.

`tls/` and the PEM files are valid but publicly shared demo credentials generated only to demonstrate the file structure. Never use them for a real deployment. For local use, generate your own certificates and JWT key pair using the commands in `TODO.md`, or copy these files without the `.example` suffix only in an isolated throwaway environment.

The local `ports.env.example` contains ports for the local Compose services. GCP firewall ports are configured separately in `deploy/ports.env.example`.

Cloud deployment secret templates are maintained in `deploy/gcp/secrets-examples/`.
