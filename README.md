# github-subscription

API skeleton for GitHub release subscriptions.

## Suggested project structure

```text
cmd/
  main.go
docs/
  swagger.yaml
internal/
  app/
  config/
  model/
  platform/
    database/
  repository/
  transport/
    httpapi/
      dto/
      handler/
migration/
Dockerfile
docker-compose.yml
```

## Layer responsibilities

- `transport/httpapi`: HTTP routing, DTOs, and handler skeletons aligned with `docs/swagger.yaml`.
- `app`: application bootstrap and HTTP server startup.
- `config`: runtime HTTP and database configuration.
- `platform/database`: Postgres bootstrap and `golang-migrate` startup migrations.
- `repository`: database access for API use cases.

## Runtime requirements

All application data is stored in PostgreSQL. On startup the service:

1. Opens the database connection.
2. Runs migrations from [migration](/Users/itkin/Developer/golang/github-subscription/migration).
3. Starts the HTTP server only after migrations succeed.

## Docker

Use [docker-compose.yml](/Users/itkin/Developer/golang/github-subscription/docker-compose.yml) to run the full stack:

```bash
docker compose up --build
```
