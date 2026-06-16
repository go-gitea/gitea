# Development

This document describes how to build Gitea from source and the day-to-day
development workflow. For prerequisites and how to obtain the code, see
[build-setup.md](build-setup.md). For running tests, see [testing.md](testing.md). For the
contribution workflow and review process, see [CONTRIBUTING.md](../CONTRIBUTING.md).

Area-specific guidelines:

- [Backend development guidelines](guidelines-backend.md)
- [Frontend development guidelines](guidelines-frontend.md)
- [Refactoring guidelines](guidelines-refactoring.md)

## Building

To build Gitea for development, run:

```bash
make build
```

No build tags are required: SQLite support is compiled in by default, which is
enough for local development. The `build` target runs two sub-targets, `frontend`
and `backend`. The `bindata` tag embeds the frontend assets into the binary and is
only needed when packaging a self-contained build, so leave it out during
development.

See `make help` for all available targets, and the workflows in
[`.github/workflows`](https://github.com/go-gitea/gitea/tree/main/.github/workflows)
to see how continuous integration builds and checks Gitea.

## Building continuously

To rebuild automatically when source files change:

```bash
# watch both frontend and backend
make watch

# or watch only the frontend (starts the Vite dev server)
make watch-frontend

# or watch only the backend (Go)
make watch-backend
```

Watching all backend source files may hit the default open-files limit on macOS or
Linux; raise it with `ulimit -n 12288` for the current shell, or in your shell
startup file to make it permanent.

## Formatting, linting and checks

Continuous integration rejects pull requests that fail formatting, linting, or
consistency checks. Format your code first:

```bash
make fmt
```

Then lint:

```bash
# lint everything
make lint
# or only one side
make lint-backend
make lint-frontend
```

Many linters can fix issues automatically with `make lint-fix` (or the scoped
`make lint-backend-fix` / `make lint-frontend-fix`). The combined consistency
checks that CI runs are available as `make checks`.

## Building and adding SVGs

SVG icons are built with `make svg`, which compiles the icon sources into
`public/assets/img/svg`. Custom icons can be added under `web_src/svg`.

## Updating the API

When you create or change API routes, you **must** update the
[Swagger](https://swagger.io/docs/specification/2-0/what-is-swagger/) documentation
using [go-swagger](https://goswagger.io/) comments. See the
[backend development guidelines](guidelines-backend.md) for how API routes,
request/response structs, and swagger definitions fit together.

Regenerate and validate the spec after changing an endpoint, then commit the
updated JSON:

```bash
make generate-swagger
make swagger-validate
```

CI verifies the committed spec is up to date with:

```bash
make swagger-check
```

## Creating new configuration options

When adding configuration options it is not enough to add them to the
`modules/setting` files. Also update
[`custom/conf/app.example.ini`](../custom/conf/app.example.ini), and document them in
the [configuration cheat sheet](https://docs.gitea.com/administration/config-cheat-sheet),
which lives in the [documentation repository](https://gitea.com/gitea/docs).

## Database migrations

If you make breaking changes to a database-persisted struct under `models/`, add a
new migration in `models/migrations/`. See [testing.md](testing.md#migration-tests)
for running the migration tests.

## Testing

For unit, integration, end-to-end, and migration tests, see [testing.md](testing.md).

## IDE configuration

### Visual Studio Code

A `launch.json` and `tasks.json` are provided in
[`contrib/development/vscode`](../contrib/development/vscode). See
[`contrib/development/README.md`](../contrib/development/README.md) for details.

### GoLand

Clicking the `Run Application` arrow on `func main()` in `/main.go` starts a
debuggable Gitea instance.

The `Output Directory` in `Run/Debug Configuration` **must** be set to the Gitea
project directory (the one containing `main.go` and `go.mod`). Otherwise the working
directory is a GoLand temporary directory, which prevents Gitea from loading dynamic
resources (such as templates) in development.

## Submitting your changes

Push your branch and open a pull request. See [CONTRIBUTING.md](../CONTRIBUTING.md)
for the review process and PR requirements. For help, join the `#Develop` channel on
[Discord](https://discord.gg/gitea).
