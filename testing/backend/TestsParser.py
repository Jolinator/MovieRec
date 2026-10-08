from pathlib import Path
from json import load
from DockerManager import DockerManager
import os
from pathlib import Path
from dotenv import load_dotenv

PROJECT_ROOT = Path(__file__).resolve().parents[2]
load_dotenv(PROJECT_ROOT / "secrets" / "ports.env")
env = os.getenv("BACKEND_PORT")
if env is None:
    raise ValueError("BACKEND_PORT is not set in the environment variables.")
BACKEND_PORT = env
BACKEND_URL = f"http://localhost:{BACKEND_PORT}"

BACKEND_TEST_PATH = Path(__file__).resolve().parent / "tests"



class TestRequest:
    def __init__(self, request_json: dict):
        self.method: str = request_json.get("method")
        self.url: str = request_json.get("url")
        self.headers: dict = request_json.get("headers")
        self.body: dict = request_json.get("body")
        self.expected_response: dict = request_json.get("expected_response")
        if any(x is None for x in [self.method, self.url, self.expected_response]):
            raise ValueError("Request JSON must contain 'method', 'url', and 'expected_response' fields.")

class Test:
    def __init__(self, name: str, fname: str, docker_manager: DockerManager):
        self.name: str = name
        self.path: Path = BACKEND_TEST_PATH / fname
        self.json: dict = load(open(self.path, "r"))
        self.services: list[str] = self.json.get("services")
        self.requests: list[TestRequest] = [TestRequest(req) for req in self.json.get("requests", [])]
        self.docker_manager: DockerManager = docker_manager
        if any(x is None for x in [self.services, self.requests]):
            raise ValueError("Test JSON must contain 'services' and 'requests' fields.")
        self.output: dict = None

    def run(self):
        self.docker_manager.set_required_services(*self.services)
        