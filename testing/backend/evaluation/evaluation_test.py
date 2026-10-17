import argparse
import csv
import os
import statistics
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import urljoin

import requests


LOAD_TEST_DIR = Path(__file__).resolve().parent
DEFAULT_RESULTS_PATH = LOAD_TEST_DIR / "csv" / "results.csv"
DEFAULT_BASE_URL = "https://34.116.153.224.nip.io"
WORKLOADS = {
    "baseline": [
        ("/movieAPI/movies?limit=20&offset=0", 1),
    ],
    "mixed": [
        ("/movieAPI/movies?limit=20&offset=0", 5),
        ("/movieAPI/genres", 2),
        ("/movieAPI/health", 1),
        ("/movieAPI/recommendations", 2),
    ],
}


@dataclass
class RequestResult:
    latency_seconds: float
    succeeded: bool


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Measure gateway throughput and latency while scaling clients."
    )
    parser.add_argument(
        "--clients",
        default="1,2,4,8,16",
        help="Comma-separated client counts to test.",
    )
    parser.add_argument(
        "--requests-per-client",
        type=int,
        default=20,
        help="Sequential requests sent by every client at each level.",
    )
    parser.add_argument(
        "--workload",
        choices=sorted(WORKLOADS),
        default="mixed",
        help="Workload to execute. Use baseline for a controlled single-endpoint test.",
    )
    parser.add_argument(
        "--base-url",
        default=DEFAULT_BASE_URL,
        help=f"Gateway URL to evaluate (default: {DEFAULT_BASE_URL}).",
    )
    parser.add_argument(
        "--insecure",
        action="store_true",
        help="Disable TLS certificate verification for deployments with self-signed certificates.",
    )
    parser.add_argument(
        "--path",
        help="Run a single custom endpoint instead of a predefined workload.",
    )
    parser.add_argument("--username", help="Username for the mixed workload.")
    parser.add_argument("--password", help="Password for the mixed workload.")
    parser.add_argument(
        "--timeout",
        type=float,
        default=10,
        help="Per-request timeout in seconds.",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=DEFAULT_RESULTS_PATH,
        help="CSV file for benchmark results.",
    )
    return parser.parse_args()


def parse_clients(value: str) -> list[int]:
    try:
        clients = [int(item.strip()) for item in value.split(",") if item.strip()]
    except ValueError as error:
        raise ValueError("--clients must contain comma-separated integers") from error
    if not clients or any(client < 1 for client in clients):
        raise ValueError("--clients must contain only positive integers")
    return clients


def load_backend_url(base_url: str) -> str:
    normalized_url = base_url.strip().rstrip("/")
    if not normalized_url.startswith(("http://", "https://")):
        raise ValueError("--base-url must start with http:// or https://")
    return normalized_url


def wait_for_backend(backend_url: str, timeout: float, verify_tls: bool) -> None:
    deadline = time.monotonic() + timeout
    health_url = urljoin(backend_url.rstrip("/") + "/", "movieAPI/health")
    while time.monotonic() < deadline:
        try:
            response = requests.get(health_url, timeout=2, verify=verify_tls)
            if response.ok:
                return
        except requests.RequestException:
            pass
        time.sleep(1)
    raise TimeoutError(f"Backend did not become available within {timeout:g}s")


def login(
    session: requests.Session,
    backend_url: str,
    username: str,
    password: str,
    timeout: float,
    verify_tls: bool,
) -> None:
    response = session.post(
        urljoin(backend_url.rstrip("/") + "/", "movieAPI/login"),
        json={"userName": username, "password": password},
        timeout=timeout,
        verify=verify_tls,
    )
    response.raise_for_status()


def build_paths(workload: list[tuple[str, int]]) -> list[str]:
    return [
        path
        for path, weight in workload
        for _ in range(weight)
    ]


def run_client(
    backend_url: str,
    paths: list[str],
    requests_per_client: int,
    timeout: float,
    start_barrier: threading.Barrier,
    credentials: tuple[str, str] | None,
    verify_tls: bool,
) -> list[RequestResult]:
    results: list[RequestResult] = []
    with requests.Session() as session:
        session.headers.update({"Accept": "application/json", "Cache-Control": "no-cache"})
        if credentials:
            login(session, backend_url, *credentials, timeout, verify_tls)
        start_barrier.wait()
        for index in range(requests_per_client):
            request_url = urljoin(
                backend_url.rstrip("/") + "/",
                paths[index % len(paths)].lstrip("/"),
            )
            started = time.perf_counter()
            try:
                response = session.get(
                    request_url,
                    timeout=timeout,
                    verify=verify_tls,
                )
                response.raise_for_status()
                succeeded = True
            except requests.RequestException:
                succeeded = False
            results.append(
                RequestResult(
                    latency_seconds=time.perf_counter() - started,
                    succeeded=succeeded,
                )
            )
    return results


def run_level(
    backend_url: str,
    paths: list[str],
    clients: int,
    requests_per_client: int,
    timeout: float,
    credentials: tuple[str, str] | None,
    verify_tls: bool,
) -> dict[str, float | int]:
    start_barrier = threading.Barrier(clients)
    started = time.perf_counter()
    with ThreadPoolExecutor(max_workers=clients) as executor:
        futures = [
            executor.submit(
                run_client,
                backend_url,
                paths,
                requests_per_client,
                timeout,
                start_barrier,
                credentials,
                verify_tls,
            )
            for _ in range(clients)
        ]
        results = [result for future in futures for result in future.result()]
    elapsed_seconds = time.perf_counter() - started
    latencies = [result.latency_seconds for result in results]
    successful_requests = sum(result.succeeded for result in results)
    total_requests = len(results)
    return {
        "clients": clients,
        "requests_per_client": requests_per_client,
        "total_requests": total_requests,
        "successful_requests": successful_requests,
        "failed_requests": total_requests - successful_requests,
        "elapsed_seconds": elapsed_seconds,
        "throughput_rps": successful_requests / elapsed_seconds,
        "mean_latency_ms": statistics.mean(latencies) * 1000,
        "p50_latency_ms": percentile(latencies, 0.50) * 1000,
        "p95_latency_ms": percentile(latencies, 0.95) * 1000,
        "p99_latency_ms": percentile(latencies, 0.99) * 1000,
    }


def percentile(values: list[float], fraction: float) -> float:
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, round((len(ordered) - 1) * fraction)))
    return ordered[index]


def write_results(path: Path, rows: list[dict[str, float | int]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fields = list(rows[0])
    with path.open("w", newline="", encoding="utf-8") as output:
        writer = csv.DictWriter(output, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)


def main() -> None:
    args = parse_args()
    clients = parse_clients(args.clients)
    if args.requests_per_client < 1:
        raise ValueError("--requests-per-client must be positive")
    if args.timeout <= 0:
        raise ValueError("--timeout must be positive")

    backend_url = load_backend_url(args.base_url)
    wait_for_backend(backend_url, args.timeout, not args.insecure)
    if args.path:
        paths = [args.path]
        workload_name = "custom"
    else:
        paths = build_paths(WORKLOADS[args.workload])
        workload_name = args.workload
    credentials = None
    if "/movieAPI/recommendations" in paths:
        username = args.username or os.getenv("LOAD_TEST_USERNAME")
        password = args.password or os.getenv("LOAD_TEST_PASSWORD")
        if not username or not password:
            raise ValueError(
                "The mixed workload requires --username and --password, "
                "or LOAD_TEST_USERNAME and LOAD_TEST_PASSWORD."
            )
        credentials = (username, password)

    rows = []
    for client_count in clients:
        row = run_level(
            backend_url,
            paths,
            client_count,
            args.requests_per_client,
            args.timeout,
            credentials,
            not args.insecure,
        )
        row["workload"] = workload_name
        rows.append(row)
        print(
            f"{client_count} clients: {row['throughput_rps']:.2f} req/s, "
            f"p95 {row['p95_latency_ms']:.2f} ms, "
            f"{row['failed_requests']} failures"
        )
    write_results(args.output, rows)
    print(f"Results written to {args.output}")


if __name__ == "__main__":
    main()
