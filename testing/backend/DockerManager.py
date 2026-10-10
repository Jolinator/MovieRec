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

class SoftDependecyManager:
    def __init__(self):
        self.allowed_services: set[str] = {
            "movies-postgres",
            "auth-postgres",
            "backend",
            "frontend",
            "prometheus",
            "grafana",
        }

        self.services: dict[str, Service] = {name: Service(name) for name in self.allowed_services}
        self.services["backend"].add_dependency(self.services["auth-postgres"])
        self.services["backend"].add_dependency(self.services["movies-postgres"])
        self.services["frontend"].add_dependency(self.services["backend"])
        self.services["prometheus"].add_dependency(self.services["backend"])
        self.services["grafana"].add_dependency(self.services["prometheus"])

    def sort_service_list(self, *services: str) -> list[str]:
        requested = set(services)
        sorted_services: list[str] = []
        visited: set[str] = set()

        def visit(service_name: str):
            if service_name in visited:
                return
            visited.add(service_name)
            for dependency in sorted(self.services[service_name].depends_on):
                if dependency in requested:
                    visit(dependency)
            sorted_services.append(service_name)

        for service in sorted(requested):
            visit(service)

        return sorted_services
        

soft_dependecy_manager: SoftDependecyManager = SoftDependecyManager()

DATABASE_STATUS_KEYS = {
    "auth-postgres": "authDBAvailable",
    "movies-postgres": "moviesDBAvailable",
}

class DockerManager:
    def __init__(self, reset_all = True):
        project_root = Path(__file__).resolve().parents[2]
        load_dotenv(project_root / "secrets" / "ports.env")
        backend_port = os.getenv("BACKEND_PORT")
        if not backend_port:
            raise ValueError("BACKEND_PORT is not set in secrets/ports.env")

        self.backend_url = f"http://localhost:{backend_port}"
        self.allowed_services: set[str] = soft_dependecy_manager.allowed_services
        self.required_services: set[str] = set()
        self.active_services: set[str] = set()
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
    ) -> dict[str, bool]:
        """Wait until requested databases and required backend imports are ready."""
        targets = self._validate_health_check(target, timeout)
        check_imports = "backend" in self.required_services
        if not targets and not check_imports:
            return {}

        url = f"{self.backend_url}/movieAPI/health"
        deadline = time.monotonic() + timeout
        while True:
            health = self._read_health_status(url)
            if health is not None and self._health_check_ready(health, targets, check_imports):
                return {service: health[service] for service in DATABASE_STATUS_KEYS}

            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise self._health_check_timeout(health, targets, check_imports, timeout)
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
            } | {"importedMoviesAvailable": health.get("importedMoviesAvailable") is True}
        except (requests.RequestException, ValueError):
            # The backend may still be starting or its health response may be unavailable.
            return None

    @staticmethod
    def _health_check_ready(
        health: dict[str, bool], targets: list[str], check_imports: bool
    ) -> bool:
        databases_ready = all(health.get(service, False) for service in targets)
        movies_import_ready = (
            not check_imports
            or not health.get("movies-postgres", False)
            or health.get("importedMoviesAvailable", False)
        )
        return databases_ready and movies_import_ready

    @staticmethod
    def _health_check_timeout(
        health: dict[str, bool] | None,
        targets: list[str],
        check_imports: bool,
        timeout: float,
    ) -> TimeoutError:
        unavailable = [
            service for service in targets
            if health is None or not health.get(service, False)
        ]
        if (
            check_imports
            and health is not None
            and health.get("movies-postgres", False)
            and not health.get("importedMoviesAvailable", False)
        ):
            unavailable.append("imported movies")
        waiting_for = ", ".join(unavailable) or "health status"
        return TimeoutError(f"Backend did not report {waiting_for} ready within {timeout:g}s")


    def set_required_services(self, *services: str) -> list[CompletedProcess]:
        results = []
        self.verify_services(*services)

        self.required_services = set(services)
        
        if "movies-postgres" in self.required_services:
            self.remove_movies()
            self.active_services.discard("movies-postgres")
            if "backend" in self.active_services:
                results.append(self.down_services("backend"))
                self.active_services.discard("backend")

        if "auth-postgres" in self.required_services:
            self.remove_auth()
            self.active_services.discard("auth-postgres")
            


        
        to_stop = self.active_services - self.required_services
        if to_stop:
            results.append(self.down_services(*to_stop))

        self.active_services -= to_stop
        to_start = self.required_services - self.active_services
        if to_start:
            results += self.up_services(*to_start)
        self.active_services |= to_start

        requested_dbs = sorted(
            self.required_services.intersection({"auth-postgres", "movies-postgres"})
        )
        if "backend" in self.required_services:
            self.health_check_dbs(target=requested_dbs)
        
        return results

    def down_services(self, *services: str  ) -> CompletedProcess:
        self.verify_services(*services)
            
        return run(
            ["docker", "compose", "down", *services],
            capture_output=True,
            text=True,
        )
        

    def up_services(self, *services: str) -> list[CompletedProcess]:
        self.verify_services(*services)
        sorted_services = soft_dependecy_manager.sort_service_list(*services)
        results = []
        for service in sorted_services:
            result = run(
                ["docker", "compose", "up", "-d", "--build", "--remove-orphans", "--wait", "--wait-timeout", "60", service],
                capture_output=True,
                text=True,
                check=True,
            )
            results.append(result)
        return results

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
        self.stop_all_services()
        self.active_services = set()
