FROM golang:1.27.1 AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

ARG SERVICE
COPY backend/ ./
RUN test -n "${SERVICE}" && CGO_ENABLED=0 GOOS=linux go build -o /out/server "./cmd/${SERVICE}"

FROM gcr.io/distroless/static-debian12

WORKDIR /app/backend
COPY --from=build /out/server /app/backend/server

ENTRYPOINT ["/app/backend/server"]
