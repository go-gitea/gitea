# Setup and requirements

This document lists the tools you need to build Gitea from source and how to get
the code. Once your environment is ready, see [development.md](development.md) for
the build and development workflow, and [testing.md](testing.md) for running tests.

For the contribution workflow and review process, see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Requirements

### Go

[Install Go](https://go.dev/doc/install) and set up your Go environment. The
required version is the one declared in [`go.mod`](../go.mod); installing the same
version your continuous integration uses avoids `gofmt` differences between Go
releases.

> [!NOTE]
> Some `make` tasks build external Go tools on demand (for example `make
> watch-backend`). To use them, the `"$GOPATH"/bin` directory must be on your
> executable `PATH`; otherwise you have to manage those tools yourself.

### Node.js and pnpm

[Install Node.js](https://nodejs.org/en/download/) to build the JavaScript and CSS
files. The minimum supported version is the one declared in
[`package.json`](../package.json) (`engines.node`); the latest LTS is recommended.

Gitea manages frontend dependencies with [pnpm](https://pnpm.io/). The `make`
targets invoke it for you, so installing pnpm manually is only needed if you want
to run `pnpm` commands directly.

### Make

Gitea uses [Make](https://www.gnu.org/software/make/) to drive builds, linting, and
tests. On Windows it can be installed via [MSYS2](https://www.msys2.org/) or
[Chocolatey](https://chocolatey.org/packages/make).

### Python with uv (optional)

Linting the templates, workflow files, and YAML requires Python tooling that Gitea
runs through [uv](https://docs.astral.sh/uv/). After installing uv, `make` creates
the environment automatically (`uv sync`); you only need this if you run
`make lint-templates`, `make lint-yaml`, or `make lint-actions` locally.

### Git LFS

The integration tests require [Git LFS](https://git-lfs.com/) to be installed.

## Getting the source code

Clone the repository:

```bash
git clone https://github.com/go-gitea/gitea
```

To contribute changes, [fork the repository](https://github.com/go-gitea/gitea) on
GitHub and add your fork as a git remote so you can push branches and open pull
requests. See GitHub's [working with forks](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks)
documentation for the details.

## Installing dependencies

Most build and test targets install the dependencies they need on their own. To
fetch everything up front, run `make deps` (or the per-group `make deps-frontend`,
`make deps-backend`, `make deps-tools`, `make deps-py`).
