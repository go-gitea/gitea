---
date: "2021-11-01T23:41:00+08:00"
title: "Guidelines for Backend Development"
slug: "guidelines-backend"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "contributing"
    name: "Guidelines for Backend"
    weight: 20
    identifier: "guidelines-backend"
---

# Guidelines for Backend Development

**Table of Contents**

{{< toc >}}

## Background

Gitea uses Golang as the backend programming language. It uses many third-party packages and also write some itself.
For example, Gitea uses [Chi](https://github.com/go-chi/chi) as basic web framework. [Xorm](https://xorm.io) is an ORM framework that is used to interact with the database.
So it's very important to manage these packages. Please take the below guidelines before you start to write backend code.

## Package Design Guideline

### Packages List

To maintain understandable code and avoid circular dependencies it is important to have a good code structure. The Gitea backend is divided into the following parts:

- `build`: Scripts to help build Gitea.
- `cmd`: All Gitea actual sub commands includes web, doctor, serv, hooks, admin and etc. `web` will start the web service. `serv` and `hooks` will be invoked by Git or OpenSSH. Other sub commands could help to maintain Gitea.
- `tests`: Common test utility functions
  - `tests/integration`: Integration tests, to test back-end regressions
  - `tests/e2e`: E2e tests, to test test front-end <> back-end compatibility and visual regressions.
- `models`: Contains the data structures used by xorm to construct database tables. It also contains functions to query and update the database. Dependencies to other Gitea code should be avoided. You can make exceptions in cases such as logging.
  - `models/db`: Basic database operations. All other `models/xxx` packages should depend on this package. The `GetEngine` function should only be invoked from `models/`.
  - `models/fixtures`: Sample data used in unit tests and integration tests. One `yml` file means one table which will be loaded into database when beginning the tests.
  - `models/migrations`: Stores database migrations between versions. PRs that change a database structure **MUST** also have a migration step.
- `modules`: Different modules to handle specific functionality in Gitea. Work in Progress: Some of them should be moved to `services`, in particular those that depend on models because they rely on the database.
  - `modules/setting`: Store all system configurations read from ini files and has been referenced by everywhere. But they should be used as function parameters when possible.
  - `modules/git`: Package to interactive with `Git` command line or Gogit package.
- `public`: Compiled frontend files (javascript, images, css, etc.)
- `routers`: Handling of server requests. As it uses other Gitea packages to serve the request, other packages (models, modules or services) must not depend on routers.
  - `routers/api` Contains routers for `/api/v1` aims to handle RESTful API requests.
  - `routers/install` Could only respond when system is in INSTALL mode (INSTALL_LOCK=false).
  - `routers/private` will only be invoked by internal sub commands, especially `serv` and `hooks`.
  - `routers/web` will handle HTTP requests from web browsers or Git SMART HTTP protocols.
- `services`: Support functions for common routing operations or command executions. Uses `models` and `modules` to handle the requests.
- `templates`: Golang templates for generating the html output.

### Package Dependencies

Since Golang doesn't support import cycles, we have to decide the package dependencies carefully. There are some levels between those packages. Below is the ideal package dependencies direction.

`cmd` -> `routers` -> `services` -> `models` -> `modules`

From left to right, left packages could depend on right packages, but right packages MUST not depend on left packages. The sub packages on the same level could depend on according this level's rules.

**NOTICE**

Why do we need database transactions outside of `models`? And how?
Some actions should allow for rollback when database record insertion/update/deletion failed.
So services must be allowed to create a database transaction. Here is some example,

```go
// services/repository/repository.go
func CreateXXXX() error {
    return db.WithTx(func(ctx context.Context) error {
        e := db.GetEngine(ctx)
        // do something, if err is returned, it will rollback automatically
        if err := issues.UpdateIssue(ctx, repoID); err != nil {
            // ...
            return err
        }
        // ...
        return nil
    })
}
```

You should **not** use `db.GetEngine(ctx)` in `services` directly, but just write a function under `models/`.
If the function will be used in the transaction, just let `context.Context` as the function's first parameter.

```go
// models/issues/issue.go
func UpdateIssue(ctx context.Context, repoID int64) error {
    e := db.GetEngine(ctx)

    // ...
}
```

### Package Name

For the top level package, use a plural as package name, i.e. `services`, `models`, for sub packages, use singular,
i.e. `services/user`, `models/repository`.

### Import Alias

Since there are some packages which use the same package name, it is possible that you find packages like `modules/user`, `models/user`, and `services/user`. When these packages are imported in one Go file, it's difficult to know which package we are using and if it's a variable name or an import name. So, we always recommend to use import aliases. To differ from package variables which are commonly in camelCase, just use **snake_case** for import aliases.
i.e. `import user_service "code.gitea.io/gitea/services/user"`

### Important Gotchas

- Never write `x.Update(exemplar)` without an explicit `WHERE` clause:
  - This will cause all rows in the table to be updated with the non-zero values of the exemplar - including IDs.
  - You should usually write `x.ID(id).Update(exemplar)`.
- If during a migration you are inserting into a table using `x.Insert(exemplar)` where the ID is preset:
  - You will need to ``SET IDENTITY_INSERT `table` ON`` for the MSSQL variant (the migration will fail otherwise)
  - However, you will also need to update the id sequence for postgres - the migration will silently pass here but later insertions will fail:
    ``SELECT setval('table_name_id_seq', COALESCE((SELECT MAX(id)+1 FROM `table_name`), 1), false)``

### Future Tasks

Currently, we are creating some refactors to do the following things:

- Correct that codes which doesn't follow the rules.
- There are too many files in `models`, so we are moving some of them into a sub package `models/xxx`.
- Some `modules` sub packages should be moved to `services` because they depend on `models`.
