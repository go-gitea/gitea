---
date: "2022-12-28T00:00:00+00:00"
title: "Conda Package Registry"
slug: "conda"
weight: 25
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Conda"
    weight: 25
    identifier: "conda"
---

# Conda Package Registry

Publish [Conda](https://docs.conda.io/en/latest/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the Conda package registry, you need to use [conda](https://docs.conda.io/projects/conda/en/stable/user-guide/install/index.html).

## Configuring the package registry

To register the package registry and provide credentials, edit your `.condarc` file:

```yaml
channel_alias: https://gitea.example.com/api/packages/{owner}/conda
channels:
  - https://gitea.example.com/api/packages/{owner}/conda
default_channels:
  - https://gitea.example.com/api/packages/{owner}/conda
```

| Placeholder  | Description |
| ------------ | ----------- |
| `owner`      | The owner of the package. |

See the [official documentation](https://conda.io/projects/conda/en/latest/user-guide/configuration/use-condarc.html) for explanations of the individual settings.

If you need to provide credentials, you may embed them as part of the channel url (`https://user:password@gitea.example.com/...`).

## Publish a package

To publish a package, perform a HTTP PUT operation with the package content in the request body.

```
PUT https://gitea.example.com/api/packages/{owner}/conda/{channel}/{filename}
```

| Placeholder  | Description |
| ------------ | ----------- |
| `owner`      | The owner of the package. |
| `channel`    | The [channel](https://conda.io/projects/conda/en/latest/user-guide/concepts/channels.html) of the package. (optional) |
| `filename`   | The name of the file. |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/package-1.0.conda \
     https://gitea.example.com/api/packages/testuser/conda/package-1.0.conda
```

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Install a package

To install a package from the package registry, execute one of the following commands:

```shell
conda install {package_name}
conda install {package_name}={package_version}
conda install -c {channel} {package_name}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `package_name`    | The package name. |
| `package_version` | The package version. |
| `channel`         | The channel of the package. (optional) |
