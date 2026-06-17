# Backend development guidelines

This document covers backend-specific architecture and contribution expectations.
For the general workflow see [CONTRIBUTING.md](../CONTRIBUTING.md), and for building
and testing see [development.md](development.md) and [testing.md](testing.md).

## Background

The backend is written in Go. Web routing is handled by
[chi](https://github.com/go-chi/chi) and database access goes through the
[XORM](https://xorm.io/) ORM. Understanding how the packages depend on each other is
essential before contributing backend code.

## Package design

### Package layout

The backend is split into top-level packages, each with a focused responsibility:

- `build`: helper scripts used at compile time
- `cmd`: subcommands such as `web`, `serv`, `hooks`, `doctor`, and admin utilities
- `models`: data structures and database operations (XORM); keeps external
  dependencies to a minimum
  - `models/db`: core database operations
  - `models/fixtures`: sample data used by tests
  - `models/migrations`: schema migration scripts
- `modules`: standalone functionality with few dependencies
  - `modules/setting`: configuration handling
  - `modules/git`: interaction with the Git command line
- `routers`: request handlers, split into `api`, `web`, `install`, and `private`
- `services`: business logic that ties routers and models together
- `templates`: Go HTML templates
- `public`: compiled frontend assets
- `tests`: integration and end-to-end test helpers

### Dependency direction

Dependencies only flow in one direction:

```text
cmd → routers → services → models → modules
```

A package on the left may import a package on its right, but never the reverse.

### Naming conventions

- Top-level packages use the plural form: `services`, `models`, `routers`.
- Subpackages use the singular form: `services/user`, `models/repository`.

When packages from different layers share a name, use a snake_case import alias to
disambiguate:

```go
import user_service "gitea.dev/services/user"
```

### Database transactions

Operations that must roll back together should run inside `db.WithTx()` (or
`db.WithTx2()` when a value must be returned), defined in `models/db/context.go`.
Functions that participate in a transaction take a `context.Context` as their first
parameter so the transaction can be propagated.

### XORM gotchas

- Never call `x.Update(exemplar)` without an explicit `WHERE` clause — it updates
  every row in the table.
- Partial table migrations must use `SyncWithOptions(IgnoreDrop...)` rather than a
  plain `Sync`.
- When inserting rows with preset IDs, MSSQL requires `SET IDENTITY_INSERT` to be
  enabled and PostgreSQL requires the sequence to be updated afterwards.

## Dependencies

Go dependencies are managed with [Go Modules](https://go.dev/ref/mod).

Pull requests should only modify `go.mod` and `go.sum` where it relates to the
change at hand, be it a bug fix or a new feature. Otherwise, these files should only
be touched by pull requests whose sole purpose is updating dependencies. Run
`make tidy` after any change to `go.mod`.

Any `go.mod` / `go.sum` update must be justified in the PR description and must be
verified by reviewers and the merger to reference an existing upstream commit.

## API v1

The API is documented with [Swagger](https://gitea.com/api/swagger) and is modelled
on [the GitHub API](https://docs.github.com/en/rest).

### GitHub API compatibility

Gitea's API should use the same endpoints and fields as the GitHub API where
possible, unless there is a good reason to deviate.

- If Gitea offers functionality GitHub does not, a new endpoint may be added.
- If Gitea exposes information the GitHub API does not, a new field may be added as
  long as it does not collide with a GitHub field.
- Existing fields should not be removed unless there is a strong reason; the same
  applies to status responses.

If you notice a problem that would require a breaking change, leave a comment in the
code for a future refactor to API v2 (which is currently not planned) rather than
breaking v1.

### Adding and maintaining API routes

- All possible results (errors, success, and failure messages) must be documented in
  the swagger comments on the route.
- Every JSON request body must be defined as a struct in `modules/structs/` and
  registered in [`routers/api/v1/swagger/options.go`](../routers/api/v1/swagger/options.go).
- Every JSON response must be defined as a struct in `modules/structs/` and
  registered with its category under [`routers/api/v1/swagger/`](../routers/api/v1/swagger).

### HTTP methods and status codes

In general, choose HTTP methods as follows:

- **GET** returns the requested object(s) with status **200 OK**.
- **POST** creates a new object (e.g. a user) and returns **201 Created** with the
  created object.
- **PUT** adds or assigns an existing object (e.g. a user to a team) and returns
  **204 No Content** with no body.
- **PATCH** edits an existing object and returns the changed object with **200 OK**.
- **DELETE** removes an object and returns **204 No Content** with no body.

### Requirements for API routes

- All parameters of endpoints that edit an object must be optional, except those
  needed to identify the object, which are required.
- Endpoints returning lists must support pagination (`page` and `limit` query
  options) and set the `X-Total-Count` header via `ctx.SetTotalCountHeader(...)`.
