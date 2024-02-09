---
date: "2021-07-20T00:00:00+00:00"
title: "Conan Package Registry"
slug: "conan"
sidebar_position: 20
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Conan"
    sidebar_position: 20
    identifier: "conan"
---

# Conan Package Registry

Publish [Conan](https://conan.io/) packages for your user or organization.

## Requirements

To work with the Conan package registry, you need to use the [conan](https://conan.io/downloads.html) command line tool to consume and publish packages.

## Configuring the package registry

To register the package registry you need to configure a new Conan remote:

```shell
conan remote add {remote} https://gitea.example.com/api/packages/{owner}/conan
conan user --remote {remote} --password {password} {username}
```

| Parameter  | Description |
| -----------| ----------- |
| `remote`   | The remote name. |
| `username` | Your Gitea username. |
| `password` | Your Gitea password. If you are using 2FA or OAuth use a [personal access token](development/api-usage.md#authentication) instead of the password. |
| `owner`    | The owner of the package. |

For example:

```shell
conan remote add gitea https://gitea.example.com/api/packages/testuser/conan
conan user --remote gitea --password password123 testuser
```

## Publish a package

Publish a Conan package by running the following command:

```shell
conan upload --remote={remote} {recipe}
```

| Parameter | Description |
| ----------| ----------- |
| `remote`  | The remote name. |
| `recipe`  | The recipe to upload. |

For example:

```shell
conan upload --remote=gitea ConanPackage/1.2@gitea/final
```

You cannot publish a file with the same name twice to a package. You must delete the existing package or file first.

The Gitea Conan package registry has full [revision](https://docs.conan.io/en/latest/versioning/revisions.html) support.

## Install a package

To install a Conan package from the package registry, execute the following command:

```shell
conan install --remote={remote} {recipe}
```

| Parameter | Description |
| ----------| ----------- |
| `remote`  | The remote name. |
| `recipe`  | The recipe to download. |

For example:

```shell
conan install --remote=gitea ConanPackage/1.2@gitea/final
```

## Supported commands

```
conan install
conan get
conan info
conan search
conan upload
conan user
conan download
conan remove
```
