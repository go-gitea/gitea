# Testing Gitea

Gitea has three kinds of automated tests: backend unit tests, integration tests, and end-to-end (e2e) tests. The default database for local test runs is SQLite (pure-Go modernc driver), so no extra services are required to get started.

## Unit tests

Unit tests live in `*_test.go` files next to the code they cover and run through `go test`. Set `GITEA_TEST_LOG_SQL=1` to log all SQL statements executed during the tests.

```bash
make test-backend
```

To run a single test, use `go test` directly or the `#` selector:

```bash
go test -run '^TestName$' ./modulepath/
make test-backend#TestName
```

Frontend unit tests run with:

```bash
make test-frontend
```

## Integration tests

Integration tests exercise Gitea against a real database. They live in `tests/integration/` and require `git lfs` to be installed. With an empty `GITEA_TEST_DATABASE` they default to SQLite:

```bash
make test-integration
```

Use `GITEA_TEST_DATABASE` to run against MySQL, PostgreSQL, or MSSQL instead. The required connection environment variables and ready-to-use Docker commands for each database are documented in [`tests/integration/README.md`](../tests/integration/README.md), which also explains how to run a single integration test.

## End-to-end tests

End-to-end tests drive a running Gitea instance with [Playwright](https://playwright.dev/):

```bash
make test-e2e
```

To run a single e2e test file, pass it via `GITEA_TEST_E2E_FLAGS`:

```bash
GITEA_TEST_E2E_FLAGS='<filepath>' make test-e2e
```

## Migration tests

If you change a database-persisted struct in `models/` you will usually need a new migration in `models/migrations/`. Run the migration tests with:

```bash
make test-migration
```

## Continuous integration

Our continuous integration runs the unit tests and runs the integration tests against every supported database, and also tests migration from several recent Gitea versions. Please submit your pull request with additional unit and integration tests as appropriate; prefer unit tests when the logic can be tested in isolation, and keep local integration and e2e tests fast (aim for sub-2s).
