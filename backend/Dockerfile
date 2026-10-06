FROM golang:1.27.1 AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/backend .

FROM gcr.io/distroless/static-debian12

WORKDIR /app/backend
COPY --from=build /out/backend /app/backend/server

EXPOSE 8080
ENTRYPOINT ["/app/backend/server"]
