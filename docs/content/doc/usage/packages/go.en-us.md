---
date: "2023-05-10T00:00:00+00:00"
title: "Go Package Registry"
slug: "go"
weight: 45
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Go"
    weight: 45
    identifier: "go"
---

# Go Package Registry

Publish Go packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Publish a package

To publish a Go package perform a HTTP `PUT` operation with the package content in the request body.
You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.
The package must follow the [documented structure](https://go.dev/ref/mod#zip-files).

```
PUT https://gitea.example.com/api/packages/{owner}/go/upload
```

| Parameter | Description |
| --------- | ----------- |
| `owner`   | The owner of the package. |

To authenticate to the package registry, you need to provide [custom HTTP headers or use HTTP Basic authentication]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}):

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.zip \
     https://gitea.example.com/api/packages/testuser/go/upload
```

If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) instead of the password.

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `201 Created`     | The package has been published. |
| `400 Bad Request` | The package is invalid. |
| `409 Conflict`    | A package with the same name exist already. |

## Install a package

To install a Go package instruct Go to use the package registry as proxy:

```shell
# use latest version
GOPROXY=https://gitea.example.com/api/packages/{owner}/go go install {package_name}
# or
GOPROXY=https://gitea.example.com/api/packages/{owner}/go go install {package_name}@latest
# use specific version
GOPROXY=https://gitea.example.com/api/packages/{owner}/go go install {package_name}@{package_version}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |
| `package_version` | The package version. |

If the owner of the packages is private you need to [provide credentials](https://go.dev/ref/mod#private-module-proxy-auth).

More information about the `GOPROXY` environment variable and how to protect against data leaks can be found in [the documentation](https://go.dev/ref/mod#private-modules).
