# Backend development

This document covers backend-specific contribution expectations. For general contribution workflow, see [CONTRIBUTING.md](../CONTRIBUTING.md).

For coding style and architecture, see also the [backend development guideline](https://docs.gitea.com/contributing/guidelines-backend) on the documentation site.

## Dependencies

Go dependencies are managed using [Go Modules](https://go.dev/cmd/go/#hdr-Module_maintenance). \
You can find more details in the [go mod documentation](https://go.dev/ref/mod) and the [Go Modules Wiki](https://github.com/golang/go/wiki/Modules).

Pull requests should only modify `go.mod` and `go.sum` where it is related to your change, be it a bugfix or a new feature. \
Apart from that, these files should only be modified by Pull Requests whose only purpose is to update dependencies.

The `go.mod`, `go.sum` update needs to be justified as part of the PR description,
and must be verified by the reviewers and/or merger to always reference
an existing upstream commit.

## API v1

The API is documented by [swagger](https://gitea.com/api/swagger) and is based on [the GitHub API](https://docs.github.com/en/rest).

### GitHub API compatibility

Gitea's API should use the same endpoints and fields as the GitHub API as far as possible, unless there are good reasons to deviate. \
If Gitea provides functionality that GitHub does not, a new endpoint can be created. \
If information is provided by Gitea that is not provided by the GitHub API, a new field can be used that doesn't collide with any GitHub fields. \
Updating an existing API should not remove existing fields unless there is a really good reason to do so. \
The same applies to status responses. If you notice a problem, feel free to leave a comment in the code for future refactoring to API v2 (which is currently not planned).

### Adding/Maintaining API routes

All expected results (errors, success, fail messages) must be documented ([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L319-L327)). \
All JSON input types must be defined as a struct in [modules/structs/](modules/structs/) ([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/modules/structs/issue.go#L76-L91)) \
and referenced in [routers/api/v1/swagger/options.go](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/swagger/options.go). \
They can then be used like [this example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L318). \
All JSON responses must be defined as a struct in [modules/structs/](modules/structs/) ([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/modules/structs/issue.go#L36-L68)) \
and referenced in its category in [routers/api/v1/swagger/](routers/api/v1/swagger/) ([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/swagger/issue.go#L11-L16)) \
They can be used like [this example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L277-L279).

### When to use what HTTP method

In general, HTTP methods are chosen as follows:

- **GET** endpoints return the requested object(s) and status **OK (200)**
- **DELETE** endpoints return the status **No Content (204)** and no content either
- **POST** endpoints are used to **create** new objects (e.g. a User) and return the status **Created (201)** and the created object
- **PUT** endpoints are used to **add/assign** existing Objects (e.g. a user to a team) and return the status **No Content (204)** and no content either
- **PATCH** endpoints are used to **edit/change** an existing object and return the changed object and the status **OK (200)**

### Requirements for API routes

All parameters of endpoints changing/editing an object must be optional (except the ones to identify the object, which are required).

Endpoints returning lists must

- support pagination (`page` & `limit` options in query)
- set `X-Total-Count` header via **SetTotalCountHeader** ([example](https://github.com/go-gitea/gitea/blob/7aae98cc5d4113f1e9918b7ee7dd09f67c189e3e/routers/api/v1/repo/issue.go#L444))
