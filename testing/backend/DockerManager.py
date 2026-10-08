from subprocess import run, CompletedProcess

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

        def visit(service_name: str):
            if service_name not in visited:
                visited.add(service_name)
                for dependency in self.services[service_name].depends_on:
                    visit(dependency)
                sorted_services.append(service_name)

        for service in services:
            visit(service)

        return sorted_services
        

soft_dependecy_manager: SoftDependecyManager = SoftDependecyManager()

class DockerManager:
    def __init__(self, reset_all = True):
        self.allowed_services: set[str] = soft_dependecy_manager.allowed_services
        self.required_services: set[str] = set()
        self.active_services: set[str] = set()
        if reset_all:
            self.reset_all()

    def verify_services(self, *services: str):
        for service in services:
            if service not in self.allowed_services:
                raise ValueError(f"Service {service} is not allowed.")
    
    def set_required_services(self, *services: str):
        self.verify_services(*services)

        self.required_services = set(services)
        
        if "movies-postgres" in self.required_services:
            self.remove_movies()
        if "auth-postgres" in self.required_services:
            self.remove_auth()

        to_stop = self.active_services - self.required_services
        self.down_services(*to_stop)
        to_start = self.required_services - self.active_services
        self.up_services(*to_start)
        self.active_services = self.required_services

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
                ["docker", "compose", "up", "-d", "--wait", "--wait-timeout", "60", service],
                capture_output=True,
                text=True,
                check=True,
            )
            results.append(result)
        return results

    def remove_movies(self) -> CompletedProcess:
        return run(
            ["docker", "compose", "down", "movies-postgres", "&&", "docker", "volume", "rm", "g21_movies-postgres-data"],
            capture_output=True,
            text=True,
        )
        

    def remove_auth(self) -> CompletedProcess:
        return run(
            ["docker", "compose", "down", "auth-postgres", "&&", "docker", "volume", "rm", "g21_auth-postgres-data"],
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
