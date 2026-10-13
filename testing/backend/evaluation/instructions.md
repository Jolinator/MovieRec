## Backend evaluation test

Run these commands from the repository root. Install the load-test dependencies
once with:

```bash
python3 -m pip install -r testing/backend/evaluation/requirements.txt
```

The test reads `BACKEND_PORT` from `secrets/ports.env` and sends requests
through the gateway.

### Baseline workload

The controlled baseline repeatedly requests:

```text
GET /movieAPI/movies?limit=20&offset=0
```

Run it with increasing numbers of concurrent clients:

```bash
python3 testing/backend/evaluation/evaluation_test.py \
  --workload baseline \
  --clients 1,2,4,8,16 \
  --requests-per-client 20
```

Each client sends the same number of sequential requests at every client level.

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
  --workload mixed \
  --clients 1,2,4,8,16 \
  --requests-per-client 20
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

The graphics are saved by default to:

```text
testing/backend/evaluation/graphics/
```

The generated files are:

- `throughput_vs_clients.png`
- `latency_vs_clients.png`

The latency graphic contains p50, p95, and p99 latency curves. Use
`--output-dir` to choose another output directory.