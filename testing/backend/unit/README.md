# Backend unit tests

Go unit tests live beside the backend packages in <code>backend/internal/</code>. They use fakes and <code>httptest</code>; they do not require a database, network, or Docker.

Run the complete unit suite from the repository root:

~~~bash
./ci-cd/test-backend-unit.sh
~~~

The script runs <code>go test ./... -count=1</code> and writes coverage to <code>testing/backend/unit/coverage.out</code>. Open the HTML report with:

~~~bash
cd backend
go tool cover -html=../testing/backend/unit/coverage.out
~~~

Integration tests are separate: <code>testing/backend/tests/</code> contains scenarios and <code>ci-cd/test-backend.sh</code> starts their Compose stack. The integration runner clears its test database volumes; do not run it against data you need to keep.

For load evaluation, follow <code>testing/backend/evaluation/instructions.md</code>.
