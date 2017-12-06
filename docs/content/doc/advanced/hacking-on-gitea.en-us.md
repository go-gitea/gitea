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

We won't cover the basics of a Golang setup within this guide. If you don't know how to get the environment up and running you should follow the official [install instructions](https://golang.org/doc/install).

If you want to contribute to Gitea you should fork the project and work on the `master` branch. There is a catch though, some internal packages are referenced by their GitHub URL. So you have to trick the Go tool to think that you work on a clone of the official repository. Start by downloading the source code as you normally would:

```
go get -d code.gitea.io/gitea
```

Now it's time to fork the [Gitea repository](https://github.com/go-gitea/gitea) on GitHub, after that you should have to switch to the source directory on the command line:

```
cd $GOPATH/src/code.gitea.io/gitea
```

To be able to create pull requests you should add your forked repository as a remote to the Gitea sources, otherwise you can not apply the changes to our repository because of lacking write permissions:

```
git remote rename origin upstream
git remote add origin git@github.com:<USERNAME>/gitea.git
git fetch --all --prune
```

You've got a working development environment for Gitea now. Take a look at the `Makefile` to get an overview about the available tasks. The most common tasks should be `make test` which will start our test environment and `make build` which will build a `gitea` binary into your working directory. Writing test cases is not mandatory to contribute, but we will be happy if you do.

That’s it! You are ready to hack on Gitea. Test your changes, push them to your repository and open a pull request.
