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
  transport/
    httpapi/
      dto/
      handler/
```

## Layer responsibilities

- `transport/httpapi`: HTTP routing, DTOs, and handler skeletons aligned with `docs/swagger.yaml`.
- `app`: application bootstrap and HTTP server startup.
- `config`: runtime HTTP configuration.
