# Developing Gitea

This document describes how to set up a local development environment and build Gitea from source. For the contribution workflow and review process, see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Installing dependencies

### Go

[Install Go](https://go.dev/doc/install) and set up your Go environment correctly. The required version is the one declared in [`go.mod`](../go.mod).

Gitea uses `gofmt` to format source code. The results of `gofmt` can differ between Go versions, so it is recommended to install the same version that our continuous integration runs.

> [!NOTE]
> When running make tasks that require external tools, such as `make watch-backend`, Gitea downloads and builds them as needed. To use them you must have the `"$GOPATH"/bin` directory on your executable path. If you don't, you will have to manage these tools yourself.

### Node.js

[Install Node.js](https://nodejs.org/en/download/), which is required to build the JavaScript and CSS files. The minimum supported version is the one declared in [`package.json`](../package.json); the latest LTS version is recommended. Gitea uses [pnpm](https://pnpm.io/) to manage frontend dependencies; the `make` targets invoke it for you, so a manual install is only needed if you want to run `pnpm` commands directly.

### Python (optional)

To lint the template files, ensure [Python](https://www.python.org/) and [Poetry](https://python-poetry.org/) are installed.

### Make

Gitea makes heavy use of Make to automate tasks and improve development. On Linux and macOS it is usually preinstalled or available from the system package manager.

#### On Windows

Make can be provided on Windows by either of these:

- [MinGW-w64](https://www.mingw-w64.org) / [MSYS2](https://www.msys2.org/).
  - MSYS2 is a collection of tools and libraries providing an easy-to-use environment for building, installing and running native Windows software; it includes MinGW-w64.
  - In MinGW-w64, the binary is called `mingw32-make.exe` instead of `make.exe`. Add the `bin` folder to `PATH`.
  - In MSYS2, you can use `make` directly. See [MSYS2 Porting](https://www.msys2.org/wiki/Porting/).
- [Chocolatey package](https://chocolatey.org/packages/make). Run `choco install make`.

> [!NOTE]
> If you are building with make from the Windows Command Prompt, you may run into issues. The prompts above (Git Bash or MinGW) are recommended. If you only have Command Prompt (or PowerShell) you can set environment variables using the [set](https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/set_1) command, e.g. `set TAGS=bindata`.

## Downloading and cloning the Gitea source code

The recommended method of obtaining the source code is by using `git clone`.

```bash
git clone https://github.com/go-gitea/gitea
```

## Forking Gitea

To contribute changes, [fork the Gitea repository](https://github.com/go-gitea/gitea) on GitHub and add your fork as a git remote so you can push branches and open pull requests. See GitHub's [working with forks](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks) documentation for the details.

## Building Gitea

See the [build from source instructions](https://docs.gitea.com/installation/install-from-source) for the full details.

The simplest recommended way to build from source for development is:

```bash
TAGS="sqlite" make build
```

The default `sqlite` tag uses the pure-Go [modernc](https://modernc.org/sqlite) driver, so no C compiler or extra tags are needed. To use the CGO-based mattn driver instead, build with `TAGS="sqlite sqlite_mattn sqlite_unlock_notify"`.

The `build` target executes both the `frontend` and `backend` sub-targets. The `bindata` tag embeds the frontend files into the binary; it is only needed for packaging a self-contained build and should be left out during development so that frontend changes are picked up without rebuilding.

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

### Working on JS and CSS

Frontend development should follow the [Guidelines for Frontend Development](https://docs.gitea.com/contributing/guidelines-frontend).

Before committing, make sure the linters pass:

```bash
make lint-frontend
```

### Building and adding SVGs

SVG icons are built using the `make svg` target, which compiles the icon sources into the output directory `public/assets/img/svg`. Custom icons can be added in the `web_src/svg` directory.

### Building the logo

The PNG and SVG versions of the Gitea logo are built from a single SVG source file `assets/logo.svg` using the `TAGS="gitea" make generate-images` target. Node.js and pnpm must be available to run it.

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

### Creating new configuration options

When creating new configuration options, it is not enough to add them to the `modules/setting` files. You should also add information to the [configuration cheat sheet](https://docs.gitea.com/administration/config-cheat-sheet), which lives in the [documentation repository](https://gitea.com/gitea/docs).

### Database migrations

If you make breaking changes to any of the database-persisted structs in the `models/` directory, you will need to add a new migration in `models/migrations/`.

## Testing

For how to run the backend, integration, e2e, and migration tests, see [docs/testing.md](testing.md).

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

To run unit tests with SQLite in GoLand, set `-tags sqlite` in `Go tool arguments` of `Run/Debug Configuration`.

## Submitting PRs

Once you're happy with your changes, push them up and open a pull request. It is recommended that you allow Gitea Managers and Owners to modify your PR branches, as we will need to update it to main before merging and may be able to help fix issues directly.

Any PR requires two approvals from the Gitea maintainers and needs to pass continuous integration. See the [CONTRIBUTING.md](../CONTRIBUTING.md) document.

If you need more help, pop on to [Discord](https://discord.gg/gitea) #Develop and chat there.

That's it! You are ready to start developing Gitea.
