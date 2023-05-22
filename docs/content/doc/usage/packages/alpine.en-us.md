---
date: "2023-03-25T00:00:00+00:00"
title: "Alpine Packages Repository"
slug: "packages/alpine"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Alpine"
    weight: 4
    identifier: "alpine"
---

# Alpine Packages Repository

Publish [Alpine](https://pkgs.alpinelinux.org/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the Alpine registry, you need to use a HTTP client like `curl` to upload and a package manager like `apk` to consume packages.

The following examples use `apk`.

## Configuring the package registry

To register the Alpine registry add the url to the list of known apk sources (`/etc/apk/repositories`):

```
https://gitea.example.com/api/packages/{owner}/alpine/<branch>/<repository>
```

| Placeholder  | Description |
| ------------ | ----------- |
| `owner`      | The owner of the packages. |
| `branch`     | The branch to use. |
| `repository` | The repository to use. |

If the registry is private, provide credentials in the url. You can use a password or a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}):

```
https://{username}:{your_password_or_token}@gitea.example.com/api/packages/{owner}/alpine/<branch>/<repository>
```

The Alpine registry files are signed with a RSA key which must be known to apk. Download the public key and store it in `/etc/apk/keys/`:

```shell
curl -JO https://gitea.example.com/api/packages/{owner}/alpine/key
```

Afterwards update the local package index:

```shell
apk update
```

## Publish a package

To publish an Alpine package (`*.apk`), perform a HTTP `PUT` operation with the package content in the request body.

```
PUT https://gitea.example.com/api/packages/{owner}/alpine/{branch}/{repository}
```

| Parameter    | Description |
| ------------ | ----------- |
| `owner`      | The owner of the package. |
| `branch`     | The branch may match the release version of the OS, ex: `v3.17`. |
| `repository` | The repository can be used [to group packages](https://wiki.alpinelinux.org/wiki/Repositories) or just `main` or similar. |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.apk \
     https://gitea.example.com/api/packages/testuser/alpine/v3.17/main
```

If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) instead of the password.
You cannot publish a file with the same name twice to a package. You must delete the existing package file first.

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `201 Created`     | The package has been published. |
| `400 Bad Request` | The package name, version, branch, repository or architecture are invalid. |
| `409 Conflict`    | A package file with the same combination of parameters exist already in the package. |

## Delete a package

To delete an Alpine package perform a HTTP `DELETE` operation. This will delete the package version too if there is no file left.

```
DELETE https://gitea.example.com/api/packages/{owner}/alpine/{branch}/{repository}/{architecture}/{filename}
```

| Parameter      | Description |
| -------------- | ----------- |
| `owner`        | The owner of the package. |
| `branch`       | The branch to use. |
| `repository`   | The repository to use. |
| `architecture` | The package architecture. |
| `filename`     | The file to delete.

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/alpine/v3.17/main/test-package-1.0.0.apk
```

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `204 No Content`  | Success |
| `404 Not Found`   | The package or file was not found. |

## Install a package

To install a package from the Alpine registry, execute the following commands:

```shell
# use latest version
apk add {package_name}
# use specific version
apk add {package_name}={package_version}
```
