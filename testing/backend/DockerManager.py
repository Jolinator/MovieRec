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
        sorted_services: list[str] = []
        visited: set[str] = set()
        services_set: set[str] = set(services)

        def visit(service_name: str):
            if service_name not in visited:
                visited.add(service_name)
                for dependency in self.services[service_name].depends_on:
                    visit(dependency)
                
            if service_name in services_set:
                sorted_services.append(service_name)

        for service in services:
            visit(service)

        return sorted_services
        

soft_dependecy_manager: SoftDependecyManager = SoftDependecyManager()

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
        """Wait until the backend reports each requested database available."""
        database_status_keys = {
            "auth-postgres": "authDBAvailable",
            "movies-postgres": "moviesDBAvailable",
        }
        targets = list(database_status_keys) if target is None else target
        unknown_targets = set(targets) - database_status_keys.keys()
        if unknown_targets:
            raise ValueError(
                f"Unsupported database health-check targets: {sorted(unknown_targets)}"
            )
        if timeout <= 0:
            raise ValueError("timeout must be greater than zero")
        if not targets:
            return {}

        url = f"{self.backend_url}/movieAPI/health"
        deadline = time.monotonic() + timeout
        latest_status: dict[str, bool] = {}

        while True:
            try:
                response = requests.get(url, timeout=2)
                response.raise_for_status()
                health = response.json()
                latest_status = {
                    service: health.get(status_key) is True
                    for service, status_key in database_status_keys.items()
                }
                if all(latest_status.get(service, False) for service in targets):
                    return latest_status
            except (requests.RequestException, ValueError):
                # The backend may still be starting, or its health response may
                # not be available yet. Retry until the shared deadline.
                pass

            remaining = deadline - time.monotonic()
            if remaining <= 0:
                unavailable = [
                    service
                    for service in targets
                    if not latest_status.get(service, False)
                ]
                raise TimeoutError(
                    "Backend did not report these databases available before "
                    f"the {timeout:g}s timeout: {', '.join(unavailable)}"
                )
            time.sleep(min(1, remaining))


    def set_required_services(self, *services: str) -> list[CompletedProcess]:
        results = []
        self.verify_services(*services)

        self.required_services = set(services)
        
        if "movies-postgres" in self.required_services:
            self.remove_movies()
            self.active_services.discard("movies-postgres")
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
        if "backend" in self.required_services and requested_dbs:
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
