# Backend unit tests

Go unit tests live next to the code (`backend/internal/<pkg>/*_test.go`) so they can
exercise unexported code. This folder is the single entry point and output location.

- Run all: `./ci-cd/test-backend-unit.sh`
- Output: `coverage.out` (git-ignored)
- Rules: no database, network or Docker; use `httptest` and fakes.
- HTML coverage: `cd backend && go tool cover -html=../testing/backend/unit/coverage.out`
- Repositories wrap `*pgxpool.Pool`, so DB-backed query code is covered by the integration tests instead.

| Package | Coverage |
|---|---|
| gateway | 96% |
| platform | 97% |
| httpx | 100% |
| database | 63% |
| recommendations | 53% |
| movies | 53% |
| auth | 52% |
| **total** | **53%** |

- API/integration tests are separate: `testing/backend/tests` + `ci-cd/test-backend.sh`.
- Throughput and latency evaluation: `testing/backend/evaluation/instructions.md`.
