---
date: "2021-07-20T00:00:00+00:00"
title: "PyPI Package Registry"
slug: "pypi"
weight: 100
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "PyPI"
    weight: 100
    identifier: "pypi"
---

# PyPI Package Registry

Publish [PyPI](https://pypi.org/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the PyPI package registry, you need to use the tools [pip](https://pypi.org/project/pip/) to consume and [twine](https://pypi.org/project/twine/) to publish packages.

## Configuring the package registry

To register the package registry you need to edit your local `~/.pypirc` file. Add

```ini
[distutils]
index-servers = gitea

[gitea]
repository = https://gitea.example.com/api/packages/{owner}/pypi
username = {username}
password = {password}
```

| Placeholder  | Description |
| ------------ | ----------- |
| `owner`      | The owner of the package. |
| `username`   | Your Gitea username. |
| `password`   | Your Gitea password. If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) instead of the password. |

## Publish a package

Publish a package by running the following command:

```shell
python3 -m twine upload --repository gitea /path/to/files/*
```

The package files have the extensions `.tar.gz` and `.whl`.

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Install a package

To install a PyPI package from the package registry, execute the following command:

```shell
pip install --index-url https://{username}:{password}@gitea.example.com/api/packages/{owner}/pypi/simple --no-deps {package_name}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `username`        | Your Gitea username. |
| `password`        | Your Gitea password or a personal access token. |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |

For example:

```shell
pip install --index-url https://testuser:password123@gitea.example.com/api/packages/testuser/pypi/simple --no-deps test_package
```

You can use `--extra-index-url` instead of `--index-url` but that makes you vulnerable to dependency confusion attacks because `pip` checks the official PyPi repository for the package before it checks the specified custom repository. Read the `pip` docs for more information.

## Supported commands

```
pip install
twine upload
```
