---
date: "2023-01-01T00:00:00+00:00"
title: "CRAN Package Registry"
slug: "cran"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "CRAN"
    sidebar_position: 35
    identifier: "cran"
---

# CRAN Package Registry

Publish [R](https://www.r-project.org/) packages to a [CRAN](https://cran.r-project.org/)-like registry for your user or organization.

## Requirements

To work with the CRAN package registry, you need to install [R](https://cran.r-project.org/).

## Configuring the package registry

To register the package registry you need to add it to `Rprofile.site`, either on the system-level, user-level (`~/.Rprofile`) or project-level:

```
options("repos" = c(getOption("repos"), c(gitea="https://gitea.example.com/api/packages/{owner}/cran")))
```

| Parameter | Description |
| --------- | ----------- |
| `owner`   | The owner of the package. |

If you need to provide credentials, you may embed them as part of the url (`https://user:password@gitea.example.com/...`).

## Publish a package

To publish a R package, perform a HTTP `PUT` operation with the package content in the request body.

Source packages:

```
PUT https://gitea.example.com/api/packages/{owner}/cran/src
```

| Parameter | Description |
| --------- | ----------- |
| `owner`   | The owner of the package. |

Binary packages:

```
PUT https://gitea.example.com/api/packages/{owner}/cran/bin?platform={platform}&rversion={rversion}
```

| Parameter  | Description |
| ---------- | ----------- |
| `owner`    | The owner of the package. |
| `platform` | The name of the platform. |
| `rversion` | The R version of the binary. |

For example:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/package.zip \
     https://gitea.example.com/api/packages/testuser/cran/bin?platform=windows&rversion=4.2
```

If you are using 2FA or OAuth use a [personal access token](development/api-usage.md#authentication) instead of the password.

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `201 Created`     | The package has been published. |
| `400 Bad Request` | The package is invalid. |
| `409 Conflict`    | A package file with the same combination of parameters exists already. |

## Install a package

To install a R package from the package registry, execute the following command:

```shell
install.packages("{package_name}")
```

| Parameter      | Description |
| -------------- | ----------- |
| `package_name` | The package name. |

For example:

```shell
install.packages("testpackage")
```
