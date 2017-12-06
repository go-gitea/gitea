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

We won't cover the basics of a Golang setup within this guide. If you don't know how to get the environment up and running you should follow the official [install instructions](https://golang.org/doc/install).

**Note**: Go version 1.7 or higher is required

## Download

First of all you have to retrieve the source code, the easiest way is to simply use directly Go for that. Just call the following commands to fetch the source and to switch into the working directory.

```
go get -d -u code.gitea.io/gitea
cd $GOPATH/src/code.gitea.io/gitea
```

Now it's time to decide which version of Gitea you want to build and install. Currently there are multiple options you can choose from. If you want to build our `master` branch you can directly go ahead to the [build section](#build), this branch represents our current development version and this is not intended for production use.

If you want to build the latest stable version that acts as a development branch for the tagged releases you can see the available branches and how to checkout this branch with these commands:

```
git branch -a
git checkout v1.0
```

If you would validate a Pull Request, first your must enable this new branch : (`xyz` is the PR id, for example `2663` for [#2663](https://github.com/go-gitea/gitea/pull/2663))

```
git fetch origin pull/xyz/head:pr-xyz
```

Last but not least you can also directly build our tagged versions like `v1.0.0`, if you want to build Gitea from the source this is the suggested way for that. To use the tags you need to list the available tags and checkout a specific tag with the following commands:

```
git tag -l
git checkout v1.0.0
git checkout pr-xyz
```

## Build

Since we already bundle all required libraries to build Gitea you can continue with the build process itself. We provide various [make tasks](https://github.com/go-gitea/gitea/blob/master/Makefile) to keep the build process as simple as possible. <a href='{{< relref "doc/advanced/make.en-us.md" >}}'>See here how to get Make</a>. Depending on your requirements you possibly want to add various build tags, you can choose between these tags:

* `bindata`: With this tag you can embed all assets required to run an instance of Gitea, this makes a deployment quite easy because you don't need to care about any additional file.
* `sqlite`: With this tag you can enable support for a [SQLite3](https://sqlite.org/) database, this is only suggested for tiny Gitea installations.
* `tidb`: With this tag you can enable support for a [TiDB](https://github.com/pingcap/tidb) database, it's a quite simple file-based database comparable with SQLite.
* `pam`: With this tag you can enable support for PAM (Linux Pluggable Authentication Modules), this is useful if your users should be authenticated via your available system users.

Now it's time to build the binary, we suggest to embed the assets with the `bindata` build tag, to include the assets you also have to execute the `generate` make task, otherwise the assets are not prepared to get embedded:

```
TAGS="bindata" make generate build
```

## Test

After following the steps above you will have a `gitea` binary within your working directory, first you can test it if it works like expected and afterwards you can copy it to the destination where you want to store it. When you launch Gitea manually from your CLI you can always kill it by hitting `Ctrl + C`.

```
./gitea web
```

## Anything missing?

Are we missing anything on this page? Then feel free to reach out to us on our [Discord server](https://discord.gg/NsatcWJ), there you will get answers to any question pretty fast.
