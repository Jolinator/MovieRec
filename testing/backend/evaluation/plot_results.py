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
                "total_requests": float(row["total_requests"]),
                "successful_requests": float(row["successful_requests"]),
                "failed_requests": float(row["failed_requests"]),
                "elapsed_seconds": float(row["elapsed_seconds"]),
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
    axes = pyplot.gca()
    failure_axes = axes.twinx()
    workloads = sorted({row["workload"] for row in results})
    colors = pyplot.rcParams["axes.prop_cycle"].by_key()["color"]

    for index, workload in enumerate(workloads):
        workload_rows = sorted(
            (row for row in results if row["workload"] == workload),
            key=lambda row: row["clients"],
        )
        color = colors[index % len(colors)]
        clients = [row["clients"] for row in workload_rows]
        attempted_throughput = [
            row["total_requests"] / row["elapsed_seconds"] for row in workload_rows
        ]
        failure_rates = [
            100 * row["failed_requests"] / row["total_requests"]
            for row in workload_rows
        ]
        axes.plot(
            clients,
            attempted_throughput,
            linestyle="--",
            color=color,
            alpha=0.65,
            label=f"{workload} attempted",
        )
        axes.plot(
            clients,
            [row["throughput_rps"] for row in workload_rows],
            marker="o",
            color=color,
            label=f"{workload} successful",
        )
        failure_axes.plot(
            clients,
            failure_rates,
            marker="x",
            linestyle=":",
            color=color,
            label=f"{workload} failures",
        )
    axes.set_xlabel("Concurrent clients")
    axes.set_ylabel("Request rate (requests/second)")
    failure_axes.set_ylabel("Failed requests (%)")
    axes.set_title("Throughput, offered load, and failures as clients increase")
    axes.set_xscale("log", base=2)
    axes.grid(True, alpha=0.3)
    handles, labels = axes.get_legend_handles_labels()
    failure_handles, failure_labels = failure_axes.get_legend_handles_labels()
    axes.legend(handles + failure_handles, labels + failure_labels)
    axes.figure.tight_layout()
    axes.figure.savefig(output_dir / "throughput_vs_clients.png", dpi=200)
    pyplot.close(axes.figure)


def plot_latency(results: list[dict[str, float | str]], output_dir: Path) -> None:
    pyplot.figure()
    for workload in sorted({row["workload"] for row in results}):
        workload_rows = sorted(
            (row for row in results if row["workload"] == workload),
            key=lambda row: row["clients"],
        )
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
    pyplot.xscale("log", base=2)
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
