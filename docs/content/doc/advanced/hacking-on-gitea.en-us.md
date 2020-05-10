---
date: "2016-12-01T16:00:00+02:00"
title: "Hacking on Gitea"
slug: "hacking-on-gitea"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Hacking on Gitea"
    weight: 10
    identifier: "hacking-on-gitea"
---

# Hacking on Gitea

## Installing go and setting the GOPATH

You should [install go](https://golang.org/doc/install) and set up your go
environment correctly. In particular, it is recommended to set the `$GOPATH`
environment variable and to add the go bin directory or directories
`${GOPATH//://bin:}/bin` to the `$PATH`. See the Go wiki entry for
[GOPATH](https://github.com/golang/go/wiki/GOPATH).

Next, [install Node.js with npm](https://nodejs.org/en/download/) which is
required to build the JavaScript and CSS files. The minimum supported Node.js
version is {{< min-node-version >}} and the latest LTS version is recommended.

You will also need make.
<a href='{{< relref "doc/advanced/make.en-us.md" >}}'>(See here how to get Make)</a>

**Note**: When executing make tasks that require external tools, like
`make misspell-check`, Gitea will automatically download and build these as
necessary. To be able to use these you must have the `"$GOPATH"/bin` directory
on the executable path. If you don't add the go bin directory to the
executable path you will have to manage this yourself.

**Note 2**: Go version {{< min-go-version >}} or higher is required; however, it is important
to note that our continuous integration will check that the formatting of the
source code is not changed by `gofmt` using `make fmt-check`. Unfortunately,
the results of `gofmt` can differ by the version of `go`. It is therefore
recommended to install the version of Go that our continuous integration is
running. As of last update, it should be Go version {{< go-version >}}.

## Downloading and cloning the Gitea source code

The recommended method of obtaining the source code is by using `git clone`.

```bash
git clone https://github.com/go-gitea/gitea
```

(Since the advent of go modules, it is no longer necessary to build go projects
from within the `$GOPATH`, hence the `go get` approach is no longer recommended.)

## Forking Gitea

Download the master Gitea source code as above. Then, fork the 
[Gitea repository](https://github.com/go-gitea/gitea) on GitHub,
and either switch the git remote origin for your fork or add your fork as another remote:

```bash
# Rename original Gitea origin to upstream
git remote rename origin upstream
git remote add origin "git@github.com:$GITHUB_USERNAME/gitea.git"
git fetch --all --prune
```

or:

```bash
# Add new remote for our fork
git remote add "$FORK_NAME" "git@github.com:$GITHUB_USERNAME/gitea.git"
git fetch --all --prune
```

To be able to create pull requests, the forked repository should be added as a remote
to the Gitea sources. Otherwise, changes can't be pushed.

## Building Gitea (Basic)

Take a look at our
<a href='{{< relref "doc/installation/from-source.en-us.md" >}}'>instructions</a>
for <a href='{{< relref "doc/installation/from-source.en-us.md" >}}'>building
from source</a>.

The simplest recommended way to build from source is:

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build
```

See `make help` for all available `make` tasks. Also see [`.drone.yml`](https://github.com/go-gitea/gitea/blob/master/.drone.yml) to see how our continuous integration works.

### Formatting, code analysis and spell check

Our continuous integration will reject PRs that are not properly formatted, fail
code analysis or spell check.

You should format your code with `go fmt` using:

```bash
make fmt
```

and can test whether your changes would match the results with:

```bash
make fmt-check # which runs make fmt internally
```

**Note**: The results of `go fmt` are dependent on the version of `go` present.
You should run the same version of go that is on the continuous integration
server as mentioned above. `make fmt-check` will only check if your `go` would
format differently - this may be different from the CI server version.

You should run revive, vet and spell-check on the code with:

```bash
make revive vet misspell-check
```

### Working on JS and CSS

For simple changes, edit files in `web_src`, run the build and start the server to test:

```bash
make build && ./gitea
```

`make build` runs both `make frontend` and `make backend` which can be run individually as well as long as the `bindata` tag is not used (which compiles frontend files into the binary).

For more involved changes use the `watch-frontend` task to continuously rebuild files when their sources change. The `bindata` tag must be absent. First, build and run the backend:

```bash
make backend && ./gitea
```

With the backend running, open another terminal and run:

```bash
make watch-frontend
```

Before committing, make sure the linters pass:

```bash
make lint-frontend
```

Note: When working on frontend code, set `USE_SERVICE_WORKER` to `false` in `app.ini` to prevent undesirable caching of frontend assets.

### Building Images

To build the images, ImageMagick, `inkscape` and `zopflipng` binaries must be available in
your `PATH` to run `make generate-images`.

### Updating the API

When creating new API routes or modifying existing API routes, you **MUST**
update and/or create [Swagger](https://swagger.io/docs/specification/2-0/what-is-swagger/)
documentation for these using [go-swagger](https://goswagger.io/) comments.
The structure of these comments is described in the [specification](https://goswagger.io/use/spec.html#annotation-syntax).
If you want more information about the Swagger structure, you can look at the
[Swagger 2.0 Documentation](https://swagger.io/docs/specification/2-0/basic-structure/)
or compare with a previous PR adding a new API endpoint, e.g. [PR #5483](https://github.com/go-gitea/gitea/pull/5843/files#diff-2e0a7b644cf31e1c8ef7d76b444fe3aaR20)

You should be careful not to break the API for downstream users which depend
on a stable API. In general, this means additions are acceptable, but deletions
or fundamental changes to the API will be rejected.

Once you have created or changed an API endpoint, please regenerate the Swagger
documentation using:

```bash
make generate-swagger
```

You should validate your generated Swagger file and spell-check it with:

```bash
make swagger-validate misspell-check
```

You should commit the changed swagger JSON file. The continous integration
server will check that this has been done using:

```bash
make swagger-check
```

**Note**: Please note you should use the Swagger 2.0 documentation, not the
OpenAPI 3 documentation.

### Creating new configuration options

When creating new configuration options, it is not enough to add them to the
`modules/setting` files. You should add information to `custom/conf/app.ini`
and to the
<a href='{{< relref "doc/advanced/config-cheat-sheet.en-us.md" >}}'>configuration cheat sheet</a>
found in `docs/content/doc/advanced/config-cheat-sheet.en-us.md`

### Changing the logo

When changing the Gitea logo SVG, you will need to run and commit the results
of:

```bash
make generate-images
```

This will create the necessary Gitea favicon and others.

### Database Migrations

If you make breaking changes to any of the database persisted structs in the
`models/` directory, you will need to make a new migration. These can be found
in `models/migrations/`. You can ensure that your migrations work for the main
database types using:

```bash
make test-sqlite-migration # with sqlite switched for the appropriate database
```

## Testing

There are two types of test run by Gitea: Unit tests and Integration Tests.

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make test # Runs the unit tests
```

Unit tests will not and cannot completely test Gitea alone. Therefore, we
have written integration tests; however, these are database dependent.

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build test-sqlite
```

will run the integration tests in an sqlite environment. Integration tests
require  `git lfs` to be installed. Other database tests are available but
may need adjustment to the local environment.

Look at
[`integrations/README.md`](https://github.com/go-gitea/gitea/blob/master/integrations/README.md)
for more information and how to run a single test.

Our continuous integration will test the code passes its unit tests and that
all supported databases will pass integration test in a Docker environment.
Migration from several recent versions of Gitea will also be tested.

Please submit your PR with additional tests and integration tests as
appropriate.

## Documentation for the website

Documentation for the website is found in `docs/`. If you change this you
can test your changes to ensure that they pass continuous integration using:

```bash
# from the docs directory within Gitea
make trans-copy clean build
```

You will require a copy of [Hugo](https://gohugo.io/) to run this task. Please
note: this may generate a number of untracked git objects, which will need to
be cleaned up.

## Visual Studio Code

A `launch.json` and `tasks.json` are provided within `contrib/ide/vscode` for
Visual Studio Code. Look at
[`contrib/ide/README.md`](https://github.com/go-gitea/gitea/blob/master/contrib/ide/README.md)
for more information.

## Submitting PRs

Once you're happy with your changes, push them up and open a pull request. It
is recommended that you allow Gitea Managers and Owners to modify your PR
branches as we will need to update it to master before merging and/or may be
able to help fix issues directly.

Any PR requires two approvals from the Gitea maintainers and needs to pass the
continous integration. Take a look at our
[`CONTRIBUTING.md`](https://github.com/go-gitea/gitea/blob/master/CONTRIBUTING.md)
document.

If you need more help pop on to [Discord](https://discord.gg/gitea) #Develop
and chat there.

That's it! You are ready to hack on Gitea.
