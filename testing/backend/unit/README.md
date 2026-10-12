# Backend unit tests

Go unit tests live next to the code (`backend/internal/<pkg>/*_test.go`) so they can
exercise unexported code. This folder is the single entry point and output location.

- Run all: `./ci-cd/test-backend-unit.sh`
- Output: `coverage.out` (git-ignored)
- Rules: no database, network or Docker; use `httptest` and fakes.
- API/integration tests are separate: `testing/backend/tests` + `ci-cd/test-backend.sh`.
