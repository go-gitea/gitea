---
date: "2016-12-01T16:00:00+02:00"
title: "Installation from source"
slug: "install-from-source"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "From source"
    weight: 30
    identifier: "install-from-source"
---

# Installation from source

You should [install go](https://golang.org/doc/install) and set up your go
environment correctly. In particular, it is recommended to set the `$GOPATH`
environment variable and to add the go bin directory or directories
`${GOPATH//://bin:}/bin` to the `$PATH`. See the Go wiki entry for
[GOPATH](https://github.com/golang/go/wiki/GOPATH).

Next, [install Node.js with npm](https://nodejs.org/en/download/) which is
required to build the JavaScript and CSS files. The minimum supported Node.js
version is 10 and the latest LTS version is recommended.

**Note**: When executing make tasks that require external tools, like
`make misspell-check`, Gitea will automatically download and build these as
necessary. To be able to use these, you must have the `"$GOPATH/bin"` directory
on the executable path. If you don't add the go bin directory to the
executable path, you will have to manage this yourself.

**Note 2**: Go version 1.11 or higher is required. However, it is recommended to
obtain the same version as our continuous integration, see the advice given in
<a href='{{< relref "doc/advanced/hacking-on-gitea.en-us.md" >}}'>Hacking on
Gitea</a>

## Download

First, retrieve the source code. The easiest way is to use the Go tool. Use the
following commands to fetch the source and switch into the source directory.
Go is quite opinionated about where it expects its source code, and simply
cloning the Gitea repository to an arbitrary path is likely to lead to
problems - the fixing of which is out of scope for this document.

```bash
go get -d -u code.gitea.io/gitea
cd "$GOPATH/src/code.gitea.io/gitea"
```

Decide which version of Gitea to build and install. Currently, there are
multiple options to choose from. The `master` branch represents the current
development version. To build with master, skip to the [build section](#build).

To work with tagged releases, the following commands can be used:

```bash
git branch -a
git checkout v{{< version >}}
```

To validate a Pull Request, first enable the new branch (`xyz` is the PR id;
for example `2663` for [#2663](https://github.com/go-gitea/gitea/pull/2663)):

```bash
git fetch origin pull/xyz/head:pr-xyz
```

To build Gitea from source at a specific tagged release (like v{{< version >}}), list the
available tags and check out the specific tag.

List available tags with the following.

```bash
git tag -l
git checkout v{{< version >}}  # or git checkout pr-xyz
```

## Build

To build from source, the following programs must be present on the system:

- `go` 1.11.0 or higher, see [here](https://golang.org/dl/)
- `node` 10.0.0 or higher with `npm`, see [here](https://nodejs.org/en/download/)
- `make`, see <a href='{{< relref "doc/advanced/make.en-us.md" >}}'>here</a>

Various [make tasks](https://github.com/go-gitea/gitea/blob/master/Makefile)
are provided to keep the build process as simple as possible.

Depending on requirements, the following build tags can be included.

* `bindata`: Build a single monolithic binary, with all assets included.
* `sqlite sqlite_unlock_notify`: Enable support for a
  [SQLite3](https://sqlite.org/) database. Suggested only for tiny
  installations.
* `pam`: Enable support for PAM (Linux Pluggable Authentication Modules). Can
  be used to authenticate local users or extend authentication to methods
  available to PAM.

Bundling assets into the binary using the `bindata` build tag can make
development and testing easier, but is not ideal for a production deployment.
To include assets, add the `bindata` tag:

```bash
TAGS="bindata" make build
```

In the default release build of our continuous integration system, the build
tags are: `TAGS="bindata sqlite sqlite_unlock_notify"`. The simplest
recommended way to build from source is therefore:

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build
```

The `build` target is split into two sub-targets:

- `make backend` which requires [Go 1.11](https://golang.org/dl/) or greater.
- `make frontend` which requires [Node.js 10.0.0](https://nodejs.org/en/download/) or greater.

If pre-built frontend files are present it is possible to only build the backend:

```bash
TAGS="bindata" make backend
``

## Test

After following the steps above, a `gitea` binary will be available in the working directory.
It can be tested from this directory or moved to a directory with test data. When Gitea is
launched manually from command line, it can be killed by pressing `Ctrl + C`.

```bash
./gitea web
```

## Changing the default CustomPath, CustomConf and AppWorkPath

Gitea will search for a number of things from the `CustomPath`. By default this is
the `custom/` directory in the current working directory when running Gitea. It will also
look for its configuration file `CustomConf` in `$CustomPath/conf/app.ini`, and will use the
current working directory as the relative base path `AppWorkPath` for a number configurable
values.

These values, although useful when developing, may conflict with downstream users preferences.

One option is to use a script file to shadow the `gitea` binary and create an appropriate
environment before running Gitea. However, when building you can change these defaults
using the `LDFLAGS` environment variable for `make`. The appropriate settings are as follows

* To set the `CustomPath` use `LDFLAGS="-X \"code.gitea.io/gitea/modules/setting.CustomPath=custom-path\""`
* For `CustomConf` you should use `-X \"code.gitea.io/gitea/modules/setting.CustomConf=conf.ini\"`
* For `AppWorkPath` you should use `-X \"code.gitea.io/gitea/modules/setting.AppWorkPath=working-path\"`

Add as many of the strings with their preceding `-X` to the `LDFLAGS` variable and run `make build`
with the appropriate `TAGS` as above.

Running `gitea help` will allow you to review what the computed settings will be for your `gitea`.
