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

**Note**: When executing make tasks that require external tools, like
`make misspell-check`, Gitea will automatically download and build these as
necessary. To be able to use these, you must have the `"$GOPATH/bin"` directory
on the executable path. If you don't add the go bin directory to the
executable path, you will have to manage this yourself.

**Note 2**: Go version 1.9 or higher is required. However, it is recommended to
obtain the same version as our continuous integration, see the advice given in
<a href='{{ relref "docs/advanced/hacking-on-gitea.en-us.md" }}'>Hacking on
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
git checkout v1.0
```

To validate a Pull Request, first enable the new branch (`xyz` is the PR id;
for example `2663` for [#2663](https://github.com/go-gitea/gitea/pull/2663)):

```bash
git fetch origin pull/xyz/head:pr-xyz
```

To build Gitea from source at a specific tagged release (like v1.0.0), list the
available tags and check out the specific tag.

List available tags with the following.

```bash
git tag -l
git checkout v1.0.0  # or git checkout pr-xyz
```

## Build

Since all required libraries are already bundled in the Gitea source, it's
possible to build Gitea with no additional downloads apart from Make
<a href='{{< relref "doc/advanced/make.en-us.md" >}}'>(See here how to get Make)</a>.
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
To include assets, they must be built separately using the `generate` make
task e.g.:

```bash
TAGS="bindata" make generate build
```

In the default release build of our continuous integration system, the build
tags are: `TAGS="bindata sqlite sqlite_unlock_notify"`. The simplest
recommended way to build from source is therefore:

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make generate build
```

## Test

After following the steps above, a `gitea` binary will be available in the working directory.
It can be tested from this directory or moved to a directory with test data. When Gitea is
launched manually from command line, it can be killed by pressing `Ctrl + C`.

```bash
./gitea web
```
