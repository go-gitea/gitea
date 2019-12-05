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
version is 10 and the latest LTS version is recommended.

You will also need make.
<a href='{{< relref "doc/advanced/make.en-us.md" >}}'>(See here how to get Make)</a>

**Note**: When executing make tasks that require external tools, like
`make misspell-check`, Gitea will automatically download and build these as
necessary. To be able to use these you must have the `"$GOPATH"/bin` directory
on the executable path. If you don't add the go bin directory to the
executable path you will have to manage this yourself.

**Note 2**: Go version 1.11 or higher is required; however, it is important
to note that our continuous integration will check that the formatting of the
source code is not changed by `gofmt` using `make fmt-check`. Unfortunately,
the results of `gofmt` can differ by the version of `go`. It is therefore
recommended to install the version of go that our continuous integration is
running. At the time of writing this is Go version 1.12; however, this can be
checked by looking at the
[master `.drone.yml`](https://github.com/go-gitea/gitea/blob/master/.drone.yml)
(At the time of writing
[line 67](https://github.com/go-gitea/gitea/blob/8917d66571a95f3da232a0c27bc1300210d10fde/.drone.yml#L67)
is the relevant line - but this may change.)

## Downloading and cloning the Gitea source code

Go is quite opinionated about where it expects its source code, and simply
cloning the Gitea repository to an arbitrary path is likely to lead to
problems - the fixing of which is out of scope for this document. Further, some
internal packages are referenced using their respective GitHub URL and at
present we use `vendor/` directories.

The recommended method of obtaining the source code is by using the `go get` command:

```bash
go get -d code.gitea.io/gitea
cd "$GOPATH/src/code.gitea.io/gitea"
```

This will clone the Gitea source code to: `"$GOPATH/src/code.gitea.io/gitea"`, or if `$GOPATH`
is not set `"$HOME/go/src/code.gitea.io/gitea"`.

## Forking Gitea

As stated above, you cannot clone Gitea to an arbitrary path. Download the master Gitea source
code as above. Then, fork the [Gitea repository](https://github.com/go-gitea/gitea) on GitHub,
and either switch the git remote origin for your fork or add your fork as another remote:

```bash
# Rename original Gitea origin to upstream
cd "$GOPATH/src/code.gitea.io/gitea"
git remote rename origin upstream
git remote add origin "git@github.com:$GITHUB_USERNAME/gitea.git"
git fetch --all --prune
```

or:

```bash
# Add new remote for our fork
cd "$GOPATH/src/code.gitea.io/gitea"
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

However, there are a number of additional make tasks you should be aware of.
These are documented below but you can look at our
[`Makefile`](https://github.com/go-gitea/gitea/blob/master/Makefile) for more,
and look at our
[`.drone.yml`](https://github.com/go-gitea/gitea/blob/master/.drone.yml) to see
how our continuous integration works.

### Formatting, code analysis and spell check

Our continous integration will reject PRs that are not properly formatted, fail
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

### Working on CSS

Edit files in `web_src/less` and run the linter and build the CSS files via:

```bash
make css
```

### Working on JS

Edit files in `web_src/js`, run the linter and build the JS files via:

```bash
make js
```

Note: When working on frontend code, it is advisable to set `USE_SERVICE_WORKER` to `false` in `app.ini` which will prevent undesirable caching of frontend assets.

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

will run the integration tests in an sqlite environment. Other database tests
are available but may need adjustment to the local environment.

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
cd "$GOPATH/src/code.gitea.io/gitea/docs"
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
