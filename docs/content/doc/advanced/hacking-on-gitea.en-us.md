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

Familiarity with the existing [installation instructions](https://golang.org/doc/install)
is required for this section.

To contribute to Gitea, fork the project and work on the `master` branch.

Some internal packages are referenced using their respective Github URL. This can
become problematic. To "trick" the Go tool into thinking this is a clone from the
official repository, download the source code using "go get" and follow these instructions.

```
go get -d code.gitea.io/gitea
```

Fork the [Gitea repository](https://github.com/go-gitea/gitea) on GitHub, it should
then be possible to switch the source directory on the command line.

```
cd $GOPATH/src/code.gitea.io/gitea
```

To be able to create pull requests, the forked repository should be added as a remote
to the Gitea sources, otherwise changes can't be pushed.

```
git remote rename origin upstream
git remote add origin git@github.com:<USERNAME>/gitea.git
git fetch --all --prune
```

This should provide a working development environment for Gitea. Take a look at
the `Makefile` to get an overview about the available tasks. The most common tasks
should be `make test` which will start our test environment and `make build` which
will build a `gitea` binary into the working directory. Writing test cases is not
mandatory to contribute, but it is highly encouraged and helps developers sleep
at night.

That's it! You are ready to hack on Gitea. Test changes, push them to the repository,
and open a pull request.
