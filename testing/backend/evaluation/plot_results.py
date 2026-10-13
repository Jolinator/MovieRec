import argparse
import csv
from pathlib import Path

import matplotlib.pyplot as pyplot


LOAD_TEST_DIR = Path(__file__).resolve().parent
DEFAULT_RESULTS_PATH = LOAD_TEST_DIR / "csv" / "results.csv"
DEFAULT_OUTPUT_DIR = LOAD_TEST_DIR / "graphics"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Create throughput and latency figures from load-test results."
    )
    parser.add_argument("--input", type=Path, default=DEFAULT_RESULTS_PATH)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT_DIR)
    return parser.parse_args()


def read_results(path: Path) -> list[dict[str, float | str]]:
    with path.open(newline="", encoding="utf-8") as input_file:
        return [
            {
                "workload": row.get("workload", "baseline"),
                "clients": float(row["clients"]),
                "throughput_rps": float(row["throughput_rps"]),
                "mean_latency_ms": float(row["mean_latency_ms"]),
                "p50_latency_ms": float(row["p50_latency_ms"]),
                "p95_latency_ms": float(row["p95_latency_ms"]),
                "p99_latency_ms": float(row["p99_latency_ms"]),
            }
            for row in csv.DictReader(input_file)
        ]


def plot_throughput(results: list[dict[str, float | str]], output_dir: Path) -> None:
    pyplot.figure()
    for workload in sorted({row["workload"] for row in results}):
        workload_rows = [row for row in results if row["workload"] == workload]
        pyplot.plot(
            [row["clients"] for row in workload_rows],
            [row["throughput_rps"] for row in workload_rows],
            marker="o",
            label=workload,
        )
    pyplot.xlabel("Concurrent clients")
    pyplot.ylabel("Throughput (requests/second)")
    pyplot.title("Throughput as client count increases")
    pyplot.grid(True, alpha=0.3)
    pyplot.legend()
    pyplot.tight_layout()
    pyplot.savefig(output_dir / "throughput_vs_clients.png", dpi=200)
    pyplot.close()


def plot_latency(results: list[dict[str, float | str]], output_dir: Path) -> None:
    pyplot.figure()
    for workload in sorted({row["workload"] for row in results}):
        workload_rows = [row for row in results if row["workload"] == workload]
        clients = [row["clients"] for row in workload_rows]
        for key, percentile in (
            ("p50_latency_ms", "p50"),
            ("p95_latency_ms", "p95"),
            ("p99_latency_ms", "p99"),
        ):
            pyplot.plot(
                clients,
                [row[key] for row in workload_rows],
                marker="o",
                label=f"{workload} {percentile}",
            )
    pyplot.xlabel("Concurrent clients")
    pyplot.ylabel("Latency (milliseconds)")
    pyplot.title("Latency as client count increases")
    pyplot.grid(True, alpha=0.3)
    pyplot.legend()
    pyplot.tight_layout()
    pyplot.savefig(output_dir / "latency_vs_clients.png", dpi=200)
    pyplot.close()


def main() -> None:
    args = parse_args()
    results = read_results(args.input)
    if not results:
        raise ValueError(f"No benchmark rows found in {args.input}")
    args.output_dir.mkdir(parents=True, exist_ok=True)
    plot_throughput(results, args.output_dir)
    plot_latency(results, args.output_dir)
    print(f"Figures written to {args.output_dir}")


if __name__ == "__main__":
    main()
