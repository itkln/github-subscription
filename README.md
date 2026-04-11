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
    email/
  repository/
  service/
    notifier/
    scanner/
    subscription/
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
- `platform/email`: SMTP delivery adapter.
- `repository`: database access for API use cases.
- `service/notifier`: notification use cases and message composition.
- `service/scanner`: periodic GitHub release checks for confirmed subscriptions.
- `service/subscription`: subscription use cases and API-facing business logic.

## Current logic

### `POST /api/subscribe`

Current intended flow:

1. Decode JSON request from Swagger contract.
2. Validate email format.
3. Validate repository format as `owner/repo`.
4. Check that the `(email, repo)` pair does not already exist.
5. Generate `confirm_token` and `unsubscribe_token`.
6. Store the subscription in PostgreSQL as unconfirmed.
7. Send confirmation email with HTML template and confirmation link.
8. Return `200` when the subscription is created and the confirmation email is sent.

### `GET /api/confirm/{token}`

Current intended flow:

1. Read confirmation token from path.
2. Validate that the token is not empty.
3. Find the subscription by `confirm_token`.
4. Mark subscription as confirmed.
5. Return `200` on success, `404` if token does not exist.

### `GET /api/unsubscribe/{token}`

Current intended flow:

1. Read unsubscribe token from path.
2. Validate that the token is not empty.
3. Find the subscription by `unsubscribe_token`.
4. Delete the subscription.
5. Return `200` on success, `404` if token does not exist.

### `GET /api/subscriptions?email=...`

Current intended flow:

1. Validate email query parameter.
2. Read all confirmed subscriptions for the email from PostgreSQL.
3. Return JSON array described in Swagger.

### Notifier flow

The code is organized so notification logic is separate from transport and SMTP:

1. `service/subscription` decides when a notification must be sent.
2. `service/notifier` builds email subject/body and renders Go HTML templates.
3. `platform/email` sends the final HTML email using SMTP.

This separation is intentional so a future `scanner` service can reuse `service/notifier`
without depending on HTTP handlers or SMTP details directly.

### Scanner flow

Current monolith flow:

1. Periodically load confirmed subscriptions from the database.
2. Group or iterate by repository.
3. Check the latest GitHub release/tag.
4. Compare with `last_seen_tag`.
5. If `last_seen_tag` is empty, initialize it without sending an email.
6. If there is a new release after initialization, call `service/notifier`.
7. Update `last_seen_tag` after successful notification.

## Implementation status

Implemented now:

- PostgreSQL persistence for subscriptions
- startup migrations with `golang-migrate`
- Docker and `docker-compose.yml` for API + PostgreSQL + Mailpit
- service split between subscription logic, notifier logic, and SMTP delivery
- background scanner for confirmed subscriptions and release detection
- HTML email templates for confirmation and release notifications
- unit tests for notifier, subscription, and scanner business logic

Still expected / partially completed:

- GitHub repository existence validation for `POST /api/subscribe`
- integration tests as an optional improvement

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

Configuration now lives in .env. The application loads it for local runs, and Docker Compose uses the same file for container runtime settings.

Use [.env.example](/Users/itkin/Developer/golang/github-subscription/.env.example) as the template for your local `.env`.

This starts:
- the API service
- PostgreSQL
- Mailpit for SMTP testing at [http://localhost:8025](http://localhost:8025)
