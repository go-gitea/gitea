---
date: "2016-12-01T16:00:00+02:00"
title: "Hacking on Gitea"
slug: "hacking-on-gitea"
weight: 10
toc: false
draft: false
aliases:
  - /en-us/hacking-on-gitea
menu:
  sidebar:
    parent: "development"
    name: "Hacking on Gitea"
    weight: 10
    identifier: "hacking-on-gitea"
---

# Hacking on Gitea

**Table of Contents**

{{< toc >}}

## Quickstart

To get a quick working development environment you could use Gitpod.

[![Open in Gitpod](/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/go-gitea/gitea)

## Installing go

You should [install go](https://golang.org/doc/install) and set up your go
environment correctly.

Next, [install Node.js with npm](https://nodejs.org/en/download/) which is
required to build the JavaScript and CSS files. The minimum supported Node.js
version is {{< min-node-version >}} and the latest LTS version is recommended.

**Note**: When executing make tasks that require external tools, like
`make watch-backend`, Gitea will automatically download and build these as
necessary. To be able to use these you must have the `"$GOPATH"/bin` directory
on the executable path. If you don't add the go bin directory to the
executable path you will have to manage this yourself.

**Note 2**: Go version {{< min-go-version >}} or higher is required.
Gitea uses `gofmt` to format source code. However, the results of
`gofmt` can differ by the version of `go`. Therefore it is
recommended to install the version of Go that our continuous integration is
running. As of last update, the Go version should be {{< go-version >}}.

To lint the template files, ensure [Python](https://www.python.org/) and
[Poetry](https://python-poetry.org/) are installed.

## Installing Make

Gitea makes heavy use of Make to automate tasks and improve development. This
guide covers how to install Make.

### On Linux

Install with the package manager.

On Ubuntu/Debian:

```bash
sudo apt-get install make
```

On Fedora/RHEL/CentOS:

```bash
sudo yum install make
```

### On Windows

One of these three distributions of Make will run on Windows:

- [Single binary build](http://www.equation.com/servlet/equation.cmd?fa=make). Copy somewhere and add to `PATH`.
  - [32-bits version](http://www.equation.com/ftpdir/make/32/make.exe)
  - [64-bits version](http://www.equation.com/ftpdir/make/64/make.exe)
- [MinGW-w64](https://www.mingw-w64.org) / [MSYS2](https://www.msys2.org/).
  - MSYS2 is a collection of tools and libraries providing you with an easy-to-use environment for building, installing and running native Windows software, it includes MinGW-w64.
  - In MingGW-w64, the binary is called `mingw32-make.exe` instead of `make.exe`. Add the `bin` folder to `PATH`.
  - In MSYS2, you can use `make` directly. See [MSYS2 Porting](https://www.msys2.org/wiki/Porting/).
  - To compile Gitea with CGO_ENABLED (eg: SQLite3), you might need to use [tdm-gcc](https://jmeubank.github.io/tdm-gcc/) instead of MSYS2 gcc, because MSYS2 gcc headers lack some Windows-only CRT functions like `_beginthread`.
- [Chocolatey package](https://chocolatey.org/packages/make). Run `choco install make`

**Note**: If you are attempting to build using make with Windows Command Prompt, you may run into issues. The above prompts (Git bash, or MinGW) are recommended, however if you only have command prompt (or potentially PowerShell) you can set environment variables using the [set](https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/set_1) command, e.g. `set TAGS=bindata`.

## Downloading and cloning the Gitea source code

The recommended method of obtaining the source code is by using `git clone`.

```bash
git clone https://github.com/go-gitea/gitea
```

(Since the advent of go modules, it is no longer necessary to build go projects
from within the `$GOPATH`, hence the `go get` approach is no longer recommended.)

## Forking Gitea

Download the main Gitea source code as above. Then, fork the
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
[instructions]({{< relref "doc/installation/from-source.en-us.md" >}})
for [building from source]({{< relref "doc/installation/from-source.en-us.md" >}}).

The simplest recommended way to build from source is:

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build
```

The `build` target will execute both `frontend` and `backend` sub-targets. If the `bindata` tag is present, the frontend files will be compiled into the binary. It is recommended to leave out the tag when doing frontend development so that changes will be reflected.

See `make help` for all available `make` targets. Also see [`.drone.yml`](https://github.com/go-gitea/gitea/blob/main/.drone.yml) to see how our continuous integration works.

## Building continuously

To run and continuously rebuild when source files change:

```bash
# for both frontend and backend
make watch

# or: watch frontend files (html/js/css) only
make watch-frontend

# or: watch backend files (go) only
make watch-backend
```

On macOS, watching all backend source files may hit the default open files limit which can be increased via `ulimit -n 12288` for the current shell or in your shell startup file for all future shells.

### Formatting, code analysis and spell check

Our continuous integration will reject PRs that fail the code linters (including format check, code analysis and spell check).

You should format your code:

```bash
make fmt
```

and lint the source code:

```bash
# lint both frontend and backend code
make lint
# lint only backend code
make lint-backend
```

**Note**: The results of `gofmt` are dependent on the version of `go` present.
You should run the same version of go that is on the continuous integration
server as mentioned above.

### Working on JS and CSS

Frontend development should follow [Guidelines for Frontend Development]({{< relref "doc/contributing/guidelines-frontend.en-us.md" >}})

To build with frontend resources, either use the `watch-frontend` target mentioned above or just build once:

```bash
make build && ./gitea
```

Before committing, make sure the linters pass:

```bash
make lint-frontend
```

### Configuring local ElasticSearch instance

Start local ElasticSearch instance using docker:

```sh
mkdir -p $(pwd)/data/elasticsearch
sudo chown -R 1000:1000 $(pwd)/data/elasticsearch
docker run --rm --memory="4g" -p 127.0.0.1:9200:9200 -p 127.0.0.1:9300:9300 -e "discovery.type=single-node" -v "$(pwd)/data/elasticsearch:/usr/share/elasticsearch/data" docker.elastic.co/elasticsearch/elasticsearch:7.16.3
```

Configure `app.ini`:

```ini
[indexer]
ISSUE_INDEXER_TYPE = elasticsearch
ISSUE_INDEXER_CONN_STR = http://elastic:changeme@localhost:9200
REPO_INDEXER_ENABLED = true
REPO_INDEXER_TYPE = elasticsearch
REPO_INDEXER_CONN_STR = http://elastic:changeme@localhost:9200
```

### Building and adding SVGs

SVG icons are built using the `make svg` target which compiles the icon sources defined in `build/generate-svg.js` into the output directory `public/img/svg`. Custom icons can be added in the `web_src/svg` directory.

### Building the Logo

The PNG and SVG versions of the Gitea logo are built from a single SVG source file `assets/logo.svg` using the `TAGS="gitea" make generate-images` target. To run it, Node.js and npm must be available.

The same process can also be used to generate custom logo PNGs from a SVG source file by updating `assets/logo.svg` and running `make generate-images`. Omitting the `gitea` tag will update only the user-designated logo files.

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

You should commit the changed swagger JSON file. The continuous integration
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
[configuration cheat sheet]({{< relref "doc/administration/config-cheat-sheet.en-us.md" >}})
found in `docs/content/doc/administer/config-cheat-sheet.en-us.md`

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
make test-sqlite-migration # with SQLite switched for the appropriate database
```

## Testing

There are two types of test run by Gitea: Unit tests and Integration Tests.

### Unit Tests

Unit tests are covered by `*_test.go` in `go test` system.
You can set the environment variable `GITEA_UNIT_TESTS_LOG_SQL=1` to display all SQL statements when running the tests in verbose mode (i.e. when `GOTESTFLAGS=-v` is set).

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make test # Runs the unit tests
```

### Integration Tests

Unit tests will not and cannot completely test Gitea alone. Therefore, we
have written integration tests; however, these are database dependent.

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build test-sqlite
```

will run the integration tests in an SQLite environment. Integration tests
require `git lfs` to be installed. Other database tests are available but
may need adjustment to the local environment.

Take a look at [`tests/integration/README.md`](https://github.com/go-gitea/gitea/blob/main/tests/integration/README.md)
for more information and how to run a single test.

### Testing for a PR

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
note: this may generate a number of untracked Git objects, which will need to
be cleaned up.

## Visual Studio Code

A `launch.json` and `tasks.json` are provided within `contrib/ide/vscode` for
Visual Studio Code. Look at
[`contrib/ide/README.md`](https://github.com/go-gitea/gitea/blob/main/contrib/ide/README.md)
for more information.

## GoLand

Clicking the `Run Application` arrow on the function `func main()` in `/main.go`
can quickly start a debuggable Gitea instance.

The `Output Directory` in `Run/Debug Configuration` MUST be set to the
gitea project directory (which contains `main.go` and `go.mod`),
otherwise, the started instance's working directory is a GoLand's temporary directory
and prevents Gitea from loading dynamic resources (eg: templates) in a development environment.

To run unit tests with SQLite in GoLand, set `-tags sqlite,sqlite_unlock_notify`
in `Go tool arguments` of `Run/Debug Configuration`.

## Submitting PRs

Once you're happy with your changes, push them up and open a pull request. It
is recommended that you allow Gitea Managers and Owners to modify your PR
branches as we will need to update it to main before merging and/or may be
able to help fix issues directly.

Any PR requires two approvals from the Gitea maintainers and needs to pass the
continuous integration. Take a look at our
[`CONTRIBUTING.md`](https://github.com/go-gitea/gitea/blob/main/CONTRIBUTING.md)
document.

If you need more help pop on to [Discord](https://discord.gg/gitea) #Develop
and chat there.

That's it! You are ready to hack on Gitea.
