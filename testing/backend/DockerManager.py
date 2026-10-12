import os
import time
from pathlib import Path
from subprocess import run, CompletedProcess

import requests
from dotenv import load_dotenv

class Service:
    def __init__(self, name: str):
        self.name: str = name
        self.depends_on: set[str] = set()
        self.depended_by: set[str] = set()

    def add_dependency(self, service: "Service"):
        self.depends_on.add(service.name)
        service.depended_by.add(self.name)

allowed_services: set[str] = {
    "movies-postgres",
    "auth-postgres",
    "recommendations-postgres",
    "llama-embedding",
    "llama-generation",
    "backend",
    "frontend",
    "prometheus",
    "grafana",
}

DATABASE_STATUS_KEYS = {
    "auth-postgres": "authDBAvailable",
    "movies-postgres": "moviesDBAvailable",
    "recommendations-postgres": "recommendationsDBAvailable",
}
DATABASE_SCAN_NAMES = {
    "auth-postgres": "auth",
    "movies-postgres": "movies",
    "recommendations-postgres": "recommendations",
}

class DockerManager:
    def __init__(self, reset_all = True):
        project_root = Path(__file__).resolve().parents[2]
        load_dotenv(project_root / "secrets" / "ports.env")
        backend_port = os.getenv("BACKEND_PORT")
        if not backend_port:
            raise ValueError("BACKEND_PORT is not set in secrets/ports.env")

        self.backend_url = f"http://localhost:{backend_port}"
        self.allowed_services: set[str] = allowed_services
        self.required_services: set[str] = set()
        self.active_services: set[str] = set()
        self.defiled_dbs: set[str] = set()
        if reset_all:
            self.reset_all()

    def verify_services(self, *services: str):
        for service in services:
            if service not in self.allowed_services:
                raise ValueError(f"Service {service} is not allowed.")



    def health_check_dbs(
        self,
        target: list[str] | None = None,
        timeout: float = 30,
    ) -> None:
        """Wait until requested databases and required backend services are ready."""
        targets = self._validate_health_check(target, timeout)
        check_backend = "backend" in self.required_services
        check_recommendations = (
            check_backend and "llama-embedding" in self.required_services
        )
        if not targets and not check_backend:
            return

        url = f"{self.backend_url}/movieAPI/health"
        if targets:
            scan_targets = ",".join(DATABASE_SCAN_NAMES[target] for target in targets)
            url = f"{url}?scan={scan_targets}"
        deadline = time.monotonic() + timeout
        while True:
            health = self._read_health_status(url)
            if health is not None and self._health_check_ready(
                health, targets, check_backend, check_recommendations
            ):
                return

            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise self._health_check_timeout(
                    health, targets, check_backend, check_recommendations, timeout
                )
            time.sleep(min(1, remaining))

    @staticmethod
    def _validate_health_check(target: list[str] | None, timeout: float) -> list[str]:
        targets = list(DATABASE_STATUS_KEYS) if target is None else target
        unknown_targets = set(targets) - DATABASE_STATUS_KEYS.keys()
        if unknown_targets:
            raise ValueError(
                f"Unsupported database health-check targets: {sorted(unknown_targets)}"
            )
        if timeout <= 0:
            raise ValueError("timeout must be greater than zero")
        return targets

    @staticmethod
    def _read_health_status(url: str) -> dict[str, bool] | None:
        try:
            response = requests.get(url, timeout=2)
            response.raise_for_status()
            health = response.json()
            return {
                service: health.get(status_key) is True
                for service, status_key in DATABASE_STATUS_KEYS.items()
            } | {
                "importedMoviesAvailable": health.get("importedMoviesAvailable") is True,
                "recommendationsReady": health.get("recommendationsReady") is True,
            }
        except (requests.RequestException, ValueError):
            # The backend may still be starting or its health response may be unavailable.
            return None

    @staticmethod
    def _health_check_ready(
        health: dict[str, bool],
        targets: list[str],
        check_backend: bool,
        check_recommendations: bool,
    ) -> bool:
        databases_ready = all(health.get(service, False) for service in targets)
        movies_import_ready = (
            not check_backend
            or not health.get("movies-postgres", False)
            or health.get("importedMoviesAvailable", False)
        )
        recommendations_ready = (
            not check_recommendations or health.get("recommendationsReady", False)
        )
        return databases_ready and movies_import_ready and recommendations_ready

    @staticmethod
    def _health_check_timeout(
        health: dict[str, bool] | None,
        targets: list[str],
        check_backend: bool,
        check_recommendations: bool,
        timeout: float,
    ) -> TimeoutError:
        unavailable = [
            service for service in targets
            if health is None or not health.get(service, False)
        ]
        if (
            check_backend
            and health is not None
            and health.get("movies-postgres", False)
            and not health.get("importedMoviesAvailable", False)
        ):
            unavailable.append("imported movies")
        if check_recommendations and (
            health is None or not health.get("recommendationsReady", False)
        ):
            unavailable.append("recommendations")
        waiting_for = ", ".join(unavailable) or "health status"
        return TimeoutError(f"Backend did not report {waiting_for} ready within {timeout:g}s")


    def set_required_services(self, *services: str) -> list[CompletedProcess]:
        results = []
        self.verify_services(*services)

        self.required_services = set(services)
        
        if "movies-postgres" in self.required_services and "movies-postgres" in self.defiled_dbs:
            self.remove_movies()
            self.active_services.discard("movies-postgres")
            self.defiled_dbs.discard("movies-postgres")
            if "backend" in self.active_services:
                results.append(self.stop_services("backend"))
                self.active_services.discard("backend")

        if "auth-postgres" in self.required_services and "auth-postgres" in self.defiled_dbs:
            self.remove_auth()
            self.active_services.discard("auth-postgres")
            self.defiled_dbs.discard("auth-postgres")

        if "recommendations-postgres" in self.required_services and "recommendations-postgres" in self.defiled_dbs:
            self.remove_recommendations()
            self.active_services.discard("recommendations-postgres")
            self.defiled_dbs.discard("recommendations-postgres")
            


        
        to_stop = self.active_services - self.required_services
        if to_stop:
            results.append(self.stop_services(*to_stop))

        self.active_services -= to_stop
        to_start = self.required_services - self.active_services
        if to_start:
            results += self.up_services(*to_start)
        self.active_services |= to_start

        requested_dbs = sorted(
            self.required_services.intersection(DATABASE_STATUS_KEYS)
        )
        if "backend" in self.required_services:
            self.health_check_dbs(target=requested_dbs, timeout=300)
        
        return results

    def stop_services(self, *services: str) -> CompletedProcess:
        self.verify_services(*services)

        return run(
            ["docker", "compose", "stop", *services],
            capture_output=True,
            text=True,
        )
        

    def up_services(self, *services: str) -> list[CompletedProcess]:
        self.verify_services(*services)

        result = run(
            [
                "docker",
                "compose",
                "up",
                "-d",
                "--build",
                "--remove-orphans",
                *services,
            ],
            capture_output=True,
            text=True,
            check=True,
        )
        return [result]

    def remove_movies(self) -> CompletedProcess:
        return run(
            ["docker", "compose", "down", "--volumes", "movies-postgres"],
            capture_output=True,
            text=True,
        )
        

    def remove_auth(self) -> CompletedProcess:
        return run(
            ["docker", "compose", "down", "--volumes", "auth-postgres"],
            capture_output=True,
            text=True,
        )

    def remove_recommendations(self) -> CompletedProcess:
        return run(
            ["docker", "compose", "down", "--volumes", "recommendations-postgres"],
            capture_output=True,
            text=True,
        )

    def defile_movies(self):
        self.defiled_dbs.add("movies-postgres")

    def defile_auth(self):
        self.defiled_dbs.add("auth-postgres")

    def defile_recommendations(self):
        self.defiled_dbs.add("recommendations-postgres")
        

    def stop_all_services(self, RemoveVolumes: bool = False) -> CompletedProcess:
        if not RemoveVolumes:
            clist = ["docker", "compose", "down"]
        else:
            clist = ["docker", "compose", "down", "-v"]

        return run(
            clist,
            capture_output=True,
            text=True,
        )

    def reset_all(self) -> CompletedProcess:
        self.remove_movies()
        self.remove_auth()
        self.remove_recommendations()
        self.stop_all_services()
        self.active_services = set()
        self.defiled_dbs.clear()
