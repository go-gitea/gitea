# Hacking on Gitea

This document describes how to set up a local development environment and build Gitea from source. For the contribution workflow and review process, see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Quickstart

To get a quick working development environment you could use Gitpod.

[![Open in Gitpod](../assets/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/go-gitea/gitea)

## Installing dependencies

### Go

[Install Go](https://go.dev/doc/install) and set up your Go environment correctly. Go version 1.26 or higher is required.

Gitea uses `gofmt` to format source code. The results of `gofmt` can differ between Go versions, so it is recommended to install the same version that our continuous integration runs. As of last update, that is Go 1.26.3.

> [!NOTE]
> When running make tasks that require external tools, such as `make watch-backend`, Gitea downloads and builds them as needed. To use them you must have the `"$GOPATH"/bin` directory on your executable path. If you don't, you will have to manage these tools yourself.

### Node.js

[Install Node.js with npm](https://nodejs.org/en/download/), which is required to build the JavaScript and CSS files. The minimum supported Node.js version is 22.18.0; the latest LTS version is recommended.

### Python (optional)

To lint the template files, ensure [Python](https://www.python.org/) and [Poetry](https://python-poetry.org/) are installed.

### Make

Gitea makes heavy use of Make to automate tasks and improve development.

#### On Linux

Install with the package manager.

On Ubuntu/Debian:

```bash
sudo apt-get install make
```

On Fedora/RHEL/CentOS:

```bash
sudo yum install make
```

#### On Windows

One of these three distributions of Make will run on Windows:

- [Single binary build](http://www.equation.com/servlet/equation.cmd?fa=make). Copy somewhere and add to `PATH`.
  - [32-bit version](http://www.equation.com/ftpdir/make/32/make.exe)
  - [64-bit version](http://www.equation.com/ftpdir/make/64/make.exe)
- [MinGW-w64](https://www.mingw-w64.org) / [MSYS2](https://www.msys2.org/).
  - MSYS2 is a collection of tools and libraries providing an easy-to-use environment for building, installing and running native Windows software; it includes MinGW-w64.
  - In MinGW-w64, the binary is called `mingw32-make.exe` instead of `make.exe`. Add the `bin` folder to `PATH`.
  - In MSYS2, you can use `make` directly. See [MSYS2 Porting](https://www.msys2.org/wiki/Porting/).
  - To compile Gitea with `CGO_ENABLED` (e.g. SQLite3), you might need to use [tdm-gcc](https://jmeubank.github.io/tdm-gcc/) instead of MSYS2 gcc, because MSYS2 gcc headers lack some Windows-only CRT functions like `_beginthread`.
- [Chocolatey package](https://chocolatey.org/packages/make). Run `choco install make`.

> [!NOTE]
> If you are building with make from the Windows Command Prompt, you may run into issues. The prompts above (Git Bash or MinGW) are recommended. If you only have Command Prompt (or PowerShell) you can set environment variables using the [set](https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/set_1) command, e.g. `set TAGS=bindata`.

## Downloading and cloning the Gitea source code

The recommended method of obtaining the source code is by using `git clone`.

```bash
git clone https://github.com/go-gitea/gitea
```

> [!NOTE]
> Since the advent of Go modules, it is no longer necessary to build Go projects from within `$GOPATH`, so the `go get` approach is no longer recommended.

## Forking Gitea

Download the main Gitea source code as above. Then fork the [Gitea repository](https://github.com/go-gitea/gitea) on GitHub, and either switch the git remote origin to your fork or add your fork as another remote:

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

To be able to create pull requests, the forked repository should be added as a remote to the Gitea sources, otherwise changes can't be pushed.

## Building Gitea

See the [build from source instructions](https://docs.gitea.com/installation/install-from-source) for the full details.

The simplest recommended way to build from source is:

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build
```

The `build` target executes both the `frontend` and `backend` sub-targets. If the `bindata` tag is present, the frontend files are compiled into the binary. Leave the tag out when doing frontend development so that changes are reflected without rebuilding.

See `make help` for all available `make` targets, and the workflows in [`.github/workflows`](https://github.com/go-gitea/gitea/tree/main/.github/workflows) to see how our continuous integration works.

### Building continuously

To run and continuously rebuild when source files change:

```bash
# for both frontend and backend
make watch

# or: watch frontend files (html/js/css) only
make watch-frontend

# or: watch backend files (go) only
make watch-backend
```

On macOS, watching all backend source files may hit the default open files limit, which can be raised via `ulimit -n 12288` for the current shell or in your shell startup file for all future shells.

### Formatting, code analysis and spell check

Our continuous integration will reject PRs that fail the linters (including format check, code analysis and spell check).

Format your code:

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

> [!NOTE]
> The results of `gofmt` depend on the version of Go present. Run the same version of Go that the continuous integration server uses, as mentioned above.

### Working on JS and CSS

Frontend development should follow the [Guidelines for Frontend Development](https://docs.gitea.com/contributing/guidelines-frontend).

To build with frontend resources, either use the `watch-frontend` target mentioned above or just build once:

```bash
make build && ./gitea
```

Before committing, make sure the linters pass:

```bash
make lint-frontend
```

### Configuring a local ElasticSearch instance

Start a local ElasticSearch instance using Docker:

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

SVG icons are built using the `make svg` target, which compiles the icon sources into the output directory `public/assets/img/svg`. Custom icons can be added in the `web_src/svg` directory.

### Building the logo

The PNG and SVG versions of the Gitea logo are built from a single SVG source file `assets/logo.svg` using the `TAGS="gitea" make generate-images` target. Node.js and npm must be available to run it.

The same process can generate custom logo PNGs from an SVG source file by updating `assets/logo.svg` and running `make generate-images`. Omitting the `gitea` tag updates only the user-designated logo files.

### Updating the API

When creating or modifying API routes, you **MUST** update and/or create [Swagger](https://swagger.io/docs/specification/2-0/what-is-swagger/) documentation for them using [go-swagger](https://goswagger.io/) comments. The structure of these comments is described in the [specification](https://goswagger.io/use/spec.html#annotation-syntax). For more information about the Swagger structure, see the [Swagger 2.0 Documentation](https://swagger.io/docs/specification/2-0/basic-structure/), or compare with a previous PR adding a new API endpoint, e.g. [PR #5483](https://github.com/go-gitea/gitea/pull/5843/files#diff-2e0a7b644cf31e1c8ef7d76b444fe3aaR20).

Be careful not to break the API for downstream users who depend on a stable API. In general, additions are acceptable, but deletions or fundamental changes to the API will be rejected.

Once you have created or changed an API endpoint, regenerate the Swagger documentation:

```bash
make generate-swagger
```

Validate your generated Swagger file:

```bash
make swagger-validate
```

Commit the changed Swagger JSON file. The continuous integration server checks that this has been done using:

```bash
make swagger-check
```

> [!NOTE]
> Use the Swagger 2.0 documentation, not the OpenAPI 3 documentation.

### Creating new configuration options

When creating new configuration options, it is not enough to add them to the `modules/setting` files. You should also add information to the [configuration cheat sheet](https://docs.gitea.com/administration/config-cheat-sheet), which lives in the [documentation repository](https://gitea.com/gitea/docs).

### Database migrations

If you make breaking changes to any of the database-persisted structs in the `models/` directory, you will need to add a new migration. These can be found in `models/migrations/`. You can ensure that your migrations work for the main database types using:

```bash
make test-sqlite-migration # switch SQLite for the appropriate database
```

## Testing

Gitea runs two types of test: unit tests and integration tests.

### Unit tests

Unit tests are covered by `*_test.go` files in the `go test` system. You can set the environment variable `GITEA_UNIT_TESTS_LOG_SQL=1` to display all SQL statements when running the tests in verbose mode (i.e. when `GOTESTFLAGS=-v` is set).

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make test # runs the unit tests
```

### Integration tests

Unit tests cannot completely test Gitea alone, so we have written integration tests; however, these are database dependent.

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build test-sqlite
```

will run the integration tests in an SQLite environment. Integration tests require `git lfs` to be installed. Other database tests are available but may need adjustment to the local environment.

See [`tests/integration/README.md`](../tests/integration/README.md) for more information and how to run a single test.

### Testing for a PR

Our continuous integration will test that the code passes its unit tests and that all supported databases pass integration tests in a Docker environment. Migration from several recent versions of Gitea is also tested.

Please submit your PR with additional unit and integration tests as appropriate.

## Documentation for the website

Documentation for the website lives in the [documentation repository](https://gitea.com/gitea/docs). The `docs/` directory in this repository holds contributor-facing documents only; if you change them you can check that they pass continuous integration using:

```bash
make lint-md
```

## Visual Studio Code

A `launch.json` and `tasks.json` are provided in [`contrib/development/vscode`](../contrib/development/vscode). See [`contrib/development/README.md`](../contrib/development/README.md) for more information.

## GoLand

Clicking the `Run Application` arrow on the function `func main()` in `/main.go` can quickly start a debuggable Gitea instance.

The `Output Directory` in `Run/Debug Configuration` MUST be set to the Gitea project directory (which contains `main.go` and `go.mod`). Otherwise the started instance's working directory is a GoLand temporary directory, which prevents Gitea from loading dynamic resources (e.g. templates) in a development environment.

To run unit tests with SQLite in GoLand, set `-tags sqlite,sqlite_unlock_notify` in `Go tool arguments` of `Run/Debug Configuration`.

## Submitting PRs

Once you're happy with your changes, push them up and open a pull request. It is recommended that you allow Gitea Managers and Owners to modify your PR branches, as we will need to update it to main before merging and may be able to help fix issues directly.

Any PR requires two approvals from the Gitea maintainers and needs to pass continuous integration. See the [CONTRIBUTING.md](../CONTRIBUTING.md) document.

If you need more help, pop on to [Discord](https://discord.gg/gitea) #Develop and chat there.

That's it! You are ready to hack on Gitea.
