## Backend evaluation test

Run these commands from the repository root. Install the load-test dependencies
once with:

```bash
python3 -m pip install -r testing/backend/evaluation/requirements.txt
```

The test sends requests through the gateway. By default, it evaluates the
current cloud deployment:

```text
https://34.116.153.224.nip.io
```

Change the default in `evaluation_test.py` (`DEFAULT_BASE_URL`) when the
deployment URL changes, or override it for one run with `--base-url`.

The current deployment uses a self-signed certificate, so include
`--insecure` when targeting it:

```bash
python3 testing/backend/evaluation/evaluation_test.py \
  --base-url https://34.116.153.224.nip.io \
  --insecure
```

Use `--insecure` only for this trusted test deployment. Certificate
verification remains enabled by default.

### Baseline workload

The controlled baseline repeatedly requests:

```text
GET /movieAPI/movies?limit=20&offset=0
```

Run it with increasing numbers of concurrent clients:

```bash
python3 testing/backend/evaluation/evaluation_test.py \
  --base-url https://34.116.153.224.nip.io \
  --insecure \
  --workload baseline \
  --clients 1,2,4,8,16 \
  --requests-per-client 20
```

At each client level, the runner creates exactly that many concurrent clients.
Every client sends the same number of sequential requests, using the same
workload distribution. This produces comparable throughput and latency
measurements as the number of request-generating clients increases.

To target a different deployment without changing the script:

```bash
python3 testing/backend/evaluation/evaluation_test.py \
  --base-url https://another-deployment.example.com \
  --workload baseline \
  --clients 1,2,4,8,16 \
  --requests-per-client 20
```

### Mixed workload

The mixed workload uses this fixed distribution:

- 50% movie catalogue requests
- 20% genre requests
- 10% health requests
- 20% personalized recommendation requests

Personalized recommendations require an authenticated session. Set the
credentials before running:

```bash
export LOAD_TEST_USERNAME="existing-user"
export LOAD_TEST_PASSWORD="existing-password"
```

Then run:

```bash
python3 testing/backend/evaluation/evaluation_test.py \
  --base-url https://34.116.153.224.nip.io \
  --insecure \
  --workload mixed \
  --clients 1,2,4,8,16 \
  --requests-per-client 20 \
  --timeout 90
```

The test prints throughput, p95 latency, and failures for each client level.
Results are saved by default to:

```text
testing/backend/evaluation/csv/results.csv
```

Use `--output` to save a different CSV file, for example:

```bash
python3 testing/backend/evaluation/evaluation_test.py \
  --workload baseline \
  --output testing/backend/evaluation/csv/baseline-results.csv
```

### Generate graphics

Create the throughput and latency graphics from a CSV file:

```bash
python3 testing/backend/evaluation/plot_results.py \
  --input testing/backend/evaluation/csv/results.csv
```

Use `--output` to save to a different directory, for example:

```bash
python3 testing/backend/evaluation/plot_results.py \
  --input testing/backend/evaluation/csv/baseline-results.csv \
  --output-dir testing/backend/evaluation/graphics/baseline
```

The graphics are saved by default to:

```text
testing/backend/evaluation/graphics/
```

The generated files are:

- `throughput_vs_clients.png`
- `latency_vs_clients.png`

The throughput graphic shows both attempted request rate and successful
throughput, plus the failed-request percentage on a secondary axis. This
prevents an overloaded run with many failures from appearing faster merely
because successful requests are counted. The latency graphic contains p50,
p95, and p99 latency curves. Use
`--output-dir` to choose another output directory.