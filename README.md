# Simple Go API example

This example connects the files in `backend` to make a tiny HTTP API. It uses in-memory sample data, so no database setup is needed.

## Files

- `main.go` creates the store, registers routes, and starts the server.
- `endpoints.go` connects URL paths to handler functions.
- `api_logic.go` handles requests and returns JSON.
- `sql_connection.go` provides sample data through a small in-memory store.

## Run it

Install Go, then open a terminal in the project folder and run:

```powershell
cd backend
go run .
```

The API starts at `http://localhost:8080`. Try these URLs in a browser:

- `http://localhost:8080/api/hello` returns `{"message":"Hello from the API!"}`
- `http://localhost:8080/api/items` returns `["apple","banana","orange"]`

Stop the server with **Ctrl+C**. The sample data resets each time the server restarts.
