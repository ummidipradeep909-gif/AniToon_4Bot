# Load, abuse and recovery test suite

The integration tests use isolated resources and never call Telegram or an external AI provider.

## Local prerequisites

- Go 1.24+
- Python 3.12+
- PostgreSQL 17
- Redis 7

Set the test-only connection strings:

```sh
export CLONE_TEST_DATABASE_URL='postgres://anitoon4:test-password@localhost:5432/anitoon4_test?sslmode=disable'
export REDIS_TEST_URL='redis://localhost:6379/15'
```

Use disposable test databases and Redis database 15. PostgreSQL integration tests create uniquely named schemas and drop them at cleanup. Redis tests use unique keys and remove them after each test.

## Commands

```sh
go test ./...
go test -race ./...
go vet ./...
(cd services/policy && python -m compileall -q app && python -m unittest discover -s tests -v)
```

Service-backed tests skip when their test URL is absent. CI provisions PostgreSQL 17 and Redis 7 and supplies both URLs, so these checks run in CI.

## Current coverage

- Concurrent Redis sliding-window flood bursts and bot/group/user counter separation.
- Redis client reconnect after server-side pooled connection termination.
- Concurrent PostgreSQL clone registration under a shared quota, persisted owner isolation and pool recovery after backend termination.
- AI timeout, malformed/non-success/oversized responses, Unicode request truncation and concurrent Python provider calls.
- Fuzz targets for flood/raid command parsing and blacklist phrase normalization.

These tests exercise service primitives and persistence boundaries. They do not simulate Telegram's live update delivery, real Telegram moderation APIs, process-level signal delivery, or multi-host clone ownership. Those require a controlled Telegram test bot or a separately provisioned staging environment and are not represented as covered here.
