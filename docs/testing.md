# Testing

Gitea has four kinds of automated tests: backend unit tests, integration tests,
end-to-end (e2e) tests, and migration tests. Local runs default to SQLite, so no
extra services are required to get started.

For prerequisites see [build-setup.md](build-setup.md); for the build workflow see
[development.md](development.md).

## Unit tests

Backend unit tests live in `*_test.go` files next to the code they cover. Set
`GITEA_TEST_LOG_SQL=1` to log all SQL statements executed during the tests.

```bash
make test-backend
```

To run a single backend test, use `go test` directly or the `#` selector:

```bash
go test -run '^TestName$' ./modulepath/
make test-backend#TestName
```

Frontend unit tests run with [Vitest](https://vitest.dev/):

```bash
make test-frontend
# single file:
pnpm exec vitest <path-filter>
```

## Integration tests

Integration tests exercise Gitea against a real database. They live in
`tests/integration/` and require [Git LFS](https://git-lfs.com/) to be installed.
The database is selected with `GITEA_TEST_DATABASE`; an empty value defaults to
SQLite, which needs no external service:

```bash
make test-integration
```

Run a single integration test with the `#` selector:

```bash
make test-integration#TestName
```

If you hit errors such as a mismatched database version or SSH push failures, try a
clean rebuild first:

```bash
make clean build
```

### Running against other databases

Set `GITEA_TEST_DATABASE` together with the matching `TEST_*` connection variables.
The commands below start a throwaway database container (press `Ctrl-C` to stop and
remove it) and then run the tests against it.

#### MySQL

```bash
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" -p 3306:3306 --rm --name mysql mysql:latest
```

```bash
GITEA_TEST_DATABASE=mysql TEST_MYSQL_HOST=localhost:3306 TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-integration
```

#### PostgreSQL

PostgreSQL tests also use a MinIO container for object storage:

```bash
docker run -e "POSTGRES_DB=test" -e "POSTGRES_USER=postgres" -e "POSTGRES_PASSWORD=postgres" -p 5432:5432 --rm --name pgsql postgres:latest
docker run --rm -p 9000:9000 -e MINIO_ROOT_USER=123456 -e MINIO_ROOT_PASSWORD=12345678 --name minio bitnamilegacy/minio:2023.8.31
```

```bash
GITEA_TEST_DATABASE=pgsql TEST_MINIO_ENDPOINT=localhost:9000 TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=postgres TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-integration
```

#### MSSQL

```bash
docker run -e "ACCEPT_EULA=Y" -e "MSSQL_PID=Standard" -e "SA_PASSWORD=MwantsaSecurePassword1" -p 1433:1433 --rm --name mssql microsoft/mssql-server-linux:latest
```

```bash
GITEA_TEST_DATABASE=mssql TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=gitea_test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-integration
```

### Running the database test workflow with Gitea Runner

The CI database test jobs can be run locally with
[Gitea Runner](https://gitea.com/gitea/runner). Running every job is
resource-intensive and not recommended:

```bash
gitea-runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest
```

List the available job names, then run a single one:

```bash
gitea-runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest -l
gitea-runner exec -W ./.github/workflows/pull-db-tests.yml --event=pull_request --default-actions-url="https://github.com" -i catthehacker/ubuntu:runner-latest -j <job_name>
```

## End-to-end tests

End-to-end tests drive a running Gitea instance with [Playwright](https://playwright.dev/):

```bash
make test-e2e
```

To run a single e2e test file, pass it via `GITEA_TEST_E2E_FLAGS`:

```bash
GITEA_TEST_E2E_FLAGS='<filepath>' make test-e2e
```

Useful environment variables:

| Variable | Description |
| :--- | :--- |
| `GITEA_TEST_E2E_DEBUG` | When set, show the Gitea server output. |
| `GITEA_TEST_E2E_FLAGS` | Additional flags passed to Playwright, e.g. `--ui`. |
| `GITEA_TEST_E2E_TIMEOUT_FACTOR` | Timeout multiplier (default: 4 on CI, 1 locally). |

## Migration tests

If you change a database-persisted struct under `models/` you will usually need a
new migration in `models/migrations/`. Run the migration tests with:

```bash
make test-migration
```

## Continuous integration

CI runs the unit tests, runs the integration tests against every supported database,
and tests migration from several recent Gitea versions. Please submit your pull
request with additional unit and integration tests as appropriate. Prefer unit tests
when the logic can be tested in isolation, and keep local integration and e2e tests
fast (aim for sub-2s runtime).
