---
date: "2021-11-01T23:41:00+08:00"
title: "Guidelines for Backend Development"
slug: "guidelines-backend"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "developers"
    name: "Guidelines for Backend"
    weight: 20
    identifier: "guidelines-backend"
---

# Guidelines for Backend Development

**Table of Contents**

{{< toc >}}

## Background

Gitea uses Golang as backend programming language. It used many third-party modules and also write many. i.e. 
Gitea uses [Chi](https://github.com/go-chi/chi) as web framework and has some wrappers. Use [Xorm](https://xorm.io) as an ORM framework to operate database. 
So it's very important to manage these packages. Please take the below guidelines before you start to write backend code.

## Package Design Guideline

To maintain understandable code and avoid circular dependencies it is important to have a good structure of the code. The gitea backend code is divided into the following parts:

- build: Scripts to help build Gitea.
- cmd: Sub commands to be invoked by OpenSSH / Git. Or some sub commands to help mantain Gitea.
- integration: Integrations tests
- models: Contains the data structures used by xorm to construct database tables. It also contains supporting functions to query and update the database. Dependencies to other code in Gitea should be avoided although some modules might be needed (for example for logging).
- models/fixtures: Sample model data used in integration tests.
- models/migrations: Handling of database migrations between versions. PRs that changes a database structure shall also have a migration step.
- models/db: Basic database operations. All other `models/xxx` package should depend on this package. And `GetEngine` function should only be invoked from `models/`.
- modules: Different modules to handle specific functionality in Gitea. Some of them should be moved to `services` but not finished.
- public: Compiled frontend files (javascript, images, css, etc.)
- routers: Handling of server requests. As it uses other Gitea packages to serve the request, other packages (models, modules or services) shall not depend on routers
- services: Support functions for common routing operations. Uses models and modules to handle the request.
- templates: Golang templates for generating the html output.
- vendor: External code that Gitea depends on.

There are some levels between those packages. Below is the ideal package dependencies direction.

routers/cmd -> services -> models -> models/db -> modules

**NOTICE**

Why we need database transaction outside of `models`? And how?
Some creation should allow rollback when files create failure or database record insert failed. So we have to allow services create a
database transaction. Here is some example,

```go
// servcies/repository/repo.go
func CreateXXXX() error {}\
  ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

    // do something, if return err, it will rollback automatically when `committer.Close()` is invoked.
    if err := issues.UpdateIssue(ctx, repoID); err != nil {
        // ...
    }

    // ......

    return committer.Commit()
}
```

You should NOT use `db.GetEngine(ctx)` in `services` directly, but just write a function under `models/`. And
if the function will be used in the transaction, just let `context.Context` as the function's first parameter.

```go
// models/issues/issue.go
func UpdateIssue(ctx context.Context, repoID int64) error {
    e := db.GetEngine(ctx)

    // ......
}
```