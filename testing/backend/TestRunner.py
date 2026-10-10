import json
import logging
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any
from urllib.parse import urljoin

import requests

from DockerManager import DockerManager


TESTS_DIR = Path(__file__).resolve().parent / "tests"
REPORTS_DIR = Path(__file__).resolve().parent / "reports"


class ReportLogHandler(logging.Handler):
    def __init__(self) -> None:
        super().__init__()
        self.messages: list[str] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.messages.append(self.format(record))

    def report_text(self) -> str:
        return "\n".join(self.messages) or "No log messages were recorded."


def configure_logging() -> tuple[logging.Logger, ReportLogHandler]:
    logger = logging.getLogger("backend_tests")
    logger.setLevel(logging.INFO)
    logger.propagate = False

    for handler in logger.handlers[:]:
        logger.removeHandler(handler)
        handler.close()

    formatter = logging.Formatter(
        fmt="%(asctime)s %(levelname)s %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setFormatter(formatter)

    report_handler = ReportLogHandler()
    report_handler.setFormatter(formatter)

    logger.addHandler(console_handler)
    logger.addHandler(report_handler)
    return logger, report_handler


@dataclass(eq=False)
class TestOutput:
    status_code: int
    body: Any

    def __eq__(self, other: object) -> bool:
        return (
            isinstance(other, TestOutput)
            and self.status_code == other.status_code
            and self.body == other.body
        )


def response_matches(actual: Any, expected: Any, subset: bool = False) -> bool:
    if subset and isinstance(actual, dict) and isinstance(expected, dict):
        return all(
            key in actual and response_matches(actual[key], value, subset=True)
            for key, value in expected.items()
        )
    return actual == expected


def response_has_fields(actual: Any, fields: list[str]) -> bool:
    if not fields:
        return True
    if not isinstance(actual, list) or not actual:
        return False
    return all(
        isinstance(item, dict) and all(field in item for field in fields)
        for item in actual
    )


def response_is_non_empty(actual: Any) -> bool:
    return isinstance(actual, (list, dict, str)) and len(actual) > 0


@dataclass(eq=False)
class TestRequest:
    method: str
    url: str
    expected_output: TestOutput
    headers: dict[str, str] = field(default_factory=dict)
    cookies: dict[str, str] = field(default_factory=dict)
    expected_response_subset: bool = False
    required_response_fields: list[str] = field(default_factory=list)
    expected_response_non_empty: bool = False
    expected_cookies: dict[str, bool] = field(default_factory=dict)
    body: Any = None
    output: TestOutput | None = None
    error: str | None = None

    @classmethod
    def from_config(cls, config: dict[str, Any]) -> "TestRequest":
        method = config.get("method")
        url = config.get("url")
        expected_body = config.get("expected_response")
        expected_status = config.get("expected_status", 200)
        expected_response_subset = config.get("expected_response_subset", False)
        required_response_fields = config.get("required_response_fields", [])
        expected_response_non_empty = config.get("expected_response_non_empty", False)
        expected_cookies = config.get("expected_cookies") or {}

        if not isinstance(method, str) or not method:
            raise ValueError("Each request needs a non-empty 'method'.")
        if not isinstance(url, str) or not url:
            raise ValueError("Each request needs a non-empty 'url'.")
        if expected_body is None:
            raise ValueError("Each request needs an 'expected_response'.")
        if not isinstance(expected_status, int):
            raise ValueError("'expected_status' must be an integer.")
        if not isinstance(expected_response_subset, bool):
            raise ValueError("'expected_response_subset' must be a boolean.")
        if not isinstance(required_response_fields, list) or not all(
            isinstance(field, str) and field for field in required_response_fields
        ):
            raise ValueError("'required_response_fields' must be a list of non-empty strings.")
        if not isinstance(expected_response_non_empty, bool):
            raise ValueError("'expected_response_non_empty' must be a boolean.")
        if not isinstance(expected_cookies, dict) or not all(
            isinstance(name, str) and isinstance(present, bool)
            for name, present in expected_cookies.items()
        ):
            raise ValueError("'expected_cookies' must map cookie names to booleans.")

        return cls(
            method=method,
            url=url,
            headers=config.get("headers") or {},
            cookies=config.get("cookies") or {},
            expected_response_subset=expected_response_subset,
            required_response_fields=required_response_fields,
            expected_response_non_empty=expected_response_non_empty,
            expected_cookies=expected_cookies,
            body=config.get("body"),
            expected_output=TestOutput(expected_status, expected_body),
        )

    def send(
        self,
        logger: logging.Logger,
        session: requests.Session,
        backend_url: str,
    ) -> None:
        request_url = urljoin(backend_url.rstrip("/") + "/", self.url.lstrip("/"))
        logger.info("Request: %s %s", self.method, request_url)
        request_cookies = self.cookies or session.cookies.get_dict()

        try:
            response = session.request(
                method=self.method,
                url=request_url,
                headers=self.headers,
                cookies=request_cookies,
                json=self.body,
                timeout=10,
            )
        except requests.RequestException as error:
            self.error = str(error)
            logger.error("Request failed: %s %s (%s)", self.method, self.url, error)
            return

        try:
            response_body: Any = response.json()
        except ValueError:
            response_body = response.text

        self.output = TestOutput(response.status_code, response_body)
        session_cookie_names = {cookie.name for cookie in session.cookies}
        for name, should_exist in self.expected_cookies.items():
            if (name in session_cookie_names) != should_exist:
                expectation = "present" if should_exist else "absent"
                self.error = f"Expected cookie {name!r} to be {expectation} in the test session."
                break

        if self.success:
            logger.info(
                "Request passed: %s %s (HTTP %s)",
                self.method,
                self.url,
                response.status_code,
            )
        elif self.error:
            logger.error(
                "Request failed: %s %s (%s)", self.method, self.url, self.error
            )
        else:
            logger.error(
                "Request failed: %s %s (expected HTTP %s, received HTTP %s)",
                self.method,
                self.url,
                self.expected_output.status_code,
                response.status_code,
            )

    def __eq__(self, other: object) -> bool:
        return isinstance(other, TestOutput) and self.output == other

    @property
    def success(self) -> bool:
        return (
            self.error is None
            and self.output is not None
            and self.output.status_code == self.expected_output.status_code
            and (
                self.expected_response_non_empty
                and response_is_non_empty(self.output.body)
                or not self.expected_response_non_empty
                and response_matches(
                self.output.body,
                self.expected_output.body,
                subset=self.expected_response_subset,
                )
            )
            and response_has_fields(self.output.body, self.required_response_fields)
        )

    def format_report(self) -> str:
        if self.success:
            assert self.output is not None
            return (
                f"- **PASS** `{self.method} {self.url}` "
                f"(HTTP {self.output.status_code})"
            )

        if self.error:
            return (
                f"- **FAIL** `{self.method} {self.url}`\n"
                f"  - Request error: `{self.error}`"
            )

        if self.output is None:
            return f"- **FAIL** `{self.method} {self.url}` (no response)"

        details = [
            f"- **FAIL** `{self.method} {self.url}` (HTTP {self.output.status_code})"
        ]
        if self.output.status_code != self.expected_output.status_code:
            details.append(
                f"  - Expected status: `{self.expected_output.status_code}`"
            )
        if self.expected_response_non_empty:
            if not response_is_non_empty(self.output.body):
                details.append("  - Expected a non-empty response body")
        elif not response_matches(
                self.output.body,
                self.expected_output.body,
                subset=self.expected_response_subset,
            ):
            details.extend(
                [
                    "  - Expected body:",
                    "```json",
                    json.dumps(self.expected_output.body, indent=2, ensure_ascii=False),
                    "```",
                    "  - Actual body:",
                    "```json",
                    json.dumps(self.output.body, indent=2, ensure_ascii=False),
                    "```",
                ]
            )
        if not response_has_fields(self.output.body, self.required_response_fields):
            details.append(
                "  - Missing required response fields: "
                + ", ".join(self.required_response_fields)
            )
        return "\n".join(details)


@dataclass
class BackendTest:
    path: Path
    services: list[str]
    requests: list[TestRequest]
    docker_manager: DockerManager
    setup_error: str | None = None
    docker_duration: float = 0.0
    duration: float = 0.0

    @classmethod
    def from_file(cls, path: Path, docker_manager: DockerManager) -> "BackendTest":
        with path.open(encoding="utf-8") as test_file:
            config: dict[str, Any] = json.load(test_file)

        services = config.get("services")
        requests_config = config.get("requests")
        if not isinstance(services, list) or not all(
            isinstance(service, str) for service in services
        ):
            raise ValueError(f"{path.name} must define a list of service names.")
        if (
            not isinstance(requests_config, list)
            or not requests_config
            or not all(isinstance(request, dict) for request in requests_config)
        ):
            raise ValueError(f"{path.name} must define at least one request.")

        requests_to_run = [
            TestRequest.from_config(request_config)
            for request_config in requests_config
        ]
        return cls(path, services, requests_to_run, docker_manager)

    @property
    def success(self) -> bool:
        return self.setup_error is None and all(
            request.success for request in self.requests
        )

    def run(self, logger: logging.Logger) -> None:
        logger.info(
            "Starting test %s; requested services: %s",
            self.path.stem,
            ", ".join(self.services),
        )
        docker_start_time = time.perf_counter()
        try:
            results = self.docker_manager.set_required_services(*self.services)
        except Exception as error:
            self.setup_error = str(error)
            logger.error("Could not start services for %s: %s", self.path.stem, error)
        else:
            for result in results:
                logger.info("Docker command:%s", result.args)
            logger.info("Services are ready for %s", self.path.stem)
            test_started_at = time.perf_counter()
            with requests.Session() as session:
                for request in self.requests:
                    request.send(logger, session, self.docker_manager.backend_url)
            test_ended_at = time.perf_counter()
            self.duration = test_ended_at - test_started_at
        finally:
            self.docker_duration = time.perf_counter() - docker_start_time

        

        logger.info(
            "Finished test %s: %s (Docker %.2fs, tests %.2fs)",
            self.path.stem,
            "PASS" if self.success else "FAIL",
            self.docker_duration,
            self.duration,
        )

    def format_report(self, number: int) -> str:
        status = "PASS" if self.success else "FAIL"
        lines = [
            f"## {number}. {self.path.stem} - {status}",
            "",
            f"- Test Duration: `{self.duration:.2f}s`",
            f"- Docker Duration: `{self.docker_duration:.2f}s`",
            f"- Requested services: `{', '.join(self.services)}`",
            "",
        ]

        if self.setup_error:
            lines.append(f"Service startup failed: `{self.setup_error}`")
        else:
            lines.extend(request.format_report() for request in self.requests)
        return "\n".join(lines)


def build_report(
    tests: list[BackendTest], generated_at: datetime, logs: str, backend_url: str,
    total_duration: float, docker_duration: float,
) -> str:
    passed = sum(test.success for test in tests)
    failed = len(tests) - passed
    test_duration = sum(test.duration for test in tests)
    processing_duration = max(0.0, total_duration - docker_duration - test_duration)

    sections = [
        "# Backend test report",
        "",
        f"- Generated: `{generated_at.astimezone().isoformat(timespec='seconds')}`",
        f"- Backend: `{backend_url}`",
        f"- Result: **{passed} passed, {failed} failed** ({len(tests)} total)",
        f"- Total runtime: `{total_duration:.2f}s`",
        f"- Total Docker time: `{docker_duration:.2f}s`",
        f"- Total test runtime: `{test_duration:.2f}s`",
        f"- Processing time: `{processing_duration:.2f}s`",
    ]
    sections.extend(
        [
            "",
            "---",
            "",
            *(test.format_report(index) for index, test in enumerate(tests, 1)),
            "",
            "## Run log",
            "",
            "```text",
            logs,
            "```",
        ]
    )
    return "\n".join(sections) + "\n"


def order_tests_to_minimize_service_changes(tests: list[BackendTest]) -> list[BackendTest]:
    """Greedily run tests whose service sets are closest to the current set."""
    remaining = sorted(tests, key=lambda test: test.path.name)
    ordered: list[BackendTest] = []
    active_services: set[str] = set()

    while remaining:
        next_test = min(
            remaining,
            key=lambda test: (
                len(active_services.symmetric_difference(test.services)),
                len(set(test.services) - active_services),
                test.path.name,
            ),
        )
        ordered.append(next_test)
        active_services = set(next_test.services)
        remaining.remove(next_test)

    return ordered


def main() -> int:
    run_started_at = time.perf_counter()
    logger, report_log = configure_logging()
    generated_at = datetime.now().astimezone()
    timestamp = generated_at.strftime("%Y%m%d-%H%M%S-%f")
    report_path = REPORTS_DIR / f"backend-test-report-{timestamp}.md"
    REPORTS_DIR.mkdir(parents=True, exist_ok=True)

    test_files = sorted(TESTS_DIR.glob("test_*.json"))
    if not test_files:
        logger.error("No test files found in %s", TESTS_DIR)
        report_path.write_text(
            "# Backend test report\n\n"
            f"No test files found in `{TESTS_DIR}`.\n\n"
            "## Run log\n\n```text\n"
            f"{report_log.report_text()}\n```\n",
            encoding="utf-8",
        )
        return 1
    logger.info("Starting backend test run (%d tests)", len(test_files))

    docker_started_at = time.perf_counter()
    docker_manager = DockerManager(reset_all=True)
    docker_init_duration = time.perf_counter() - docker_started_at
    tests = [BackendTest.from_file(path, docker_manager) for path in test_files]
    tests = order_tests_to_minimize_service_changes(tests)
    for test in tests:
        test.run(logger)

    total_docker_duration = docker_init_duration + sum(
        test.docker_duration for test in tests
    )
    total_duration = time.perf_counter() - run_started_at
    total_test_duration = sum(test.duration for test in tests)
    processing_duration = max(
        0.0, total_duration - total_docker_duration - total_test_duration
    )
    passed = sum(test.success for test in tests)
    logger.info("Test summary: %d passed, %d failed", passed, len(tests) - passed)
    logger.info("Total runtime: %.2fs", total_duration)
    logger.info("Total Docker time: %.2fs", total_docker_duration)
    logger.info("Total test runtime: %.2fs", total_test_duration)
    logger.info("Processing time: %.2fs", processing_duration)
    logger.info("Writing report to %s", report_path)
    report_path.write_text(
        build_report(
            tests,
            generated_at,
            report_log.report_text(),
            docker_manager.backend_url,
            total_duration,
            total_docker_duration,
        ),
        encoding="utf-8",
    )
    return 0 if all(test.success for test in tests) else 1


if __name__ == "__main__":
    sys.exit(main())
    DockerManager.reset_all()
