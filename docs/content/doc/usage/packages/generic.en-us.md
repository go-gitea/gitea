---
date: "2021-07-20T00:00:00+00:00"
title: "Generic Package Registry"
slug: "generic"
weight: 40
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Generic"
    weight: 40
    identifier: "generic"
---

# Generic Package Registry

Publish generic files, like release binaries or other output, for your user or organization.

**Table of Contents**

{{< toc >}}

## Authenticate to the package registry

To authenticate to the Package Registry, you need to provide [custom HTTP headers or use HTTP Basic authentication]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}).

## Publish a package

To publish a generic package perform a HTTP PUT operation with the package content in the request body.
You cannot publish a file with the same name twice to a package. You must delete the existing package version first.

```
PUT https://gitea.example.com/api/packages/{owner}/generic/{package_name}/{package_version}/{file_name}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. It can contain only lowercase letters (`a-z`), uppercase letter (`A-Z`), numbers (`0-9`), dots (`.`), hyphens (`-`), pluses (`+`), or underscores (`_`). |
| `package_version` | The package version, a non-empty string without trailing or leading whitespaces. |
| `file_name`       | The filename. It can contain only lowercase letters (`a-z`), uppercase letter (`A-Z`), numbers (`0-9`), dots (`.`), hyphens (`-`), pluses (`+`), or underscores (`_`). |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.bin \
     https://gitea.example.com/api/packages/testuser/generic/test_package/1.0.0/file.bin
```

If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) instead of the password.

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `201 Created`     | The package has been published. |
| `400 Bad Request` | The package name and/or version and/or file name are invalid. |
| `409 Conflict`    | A file with the same name exist already in the package. |

## Download a package

To download a generic package perform a HTTP GET operation.

```
GET https://gitea.example.com/api/packages/{owner}/generic/{package_name}/{package_version}/{file_name}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |
| `package_version` | The package version. |
| `file_name`       | The filename. |

The file content is served in the response body. The response content type is `application/octet-stream`.

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_token_or_password \
     https://gitea.example.com/api/packages/testuser/generic/test_package/1.0.0/file.bin
```

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `200 OK`          | Success |
| `404 Not Found`   | The package or file was not found. |

## Delete a package

To delete a generic package perform a HTTP DELETE operation. This will delete all files of this version.

```
DELETE https://gitea.example.com/api/packages/{owner}/generic/{package_name}/{package_version}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |
| `package_version` | The package version. |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/generic/test_package/1.0.0
```

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `204 No Content`  | Success |
| `404 Not Found`   | The package was not found. |

## Delete a package file

To delete a file of a generic package perform a HTTP DELETE operation. This will delete the package version too if there is no file left.

```
DELETE https://gitea.example.com/api/packages/{owner}/generic/{package_name}/{package_version}/{filename}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |
| `package_version` | The package version. |
| `filename`        | The filename. |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/generic/test_package/1.0.0/file.bin
```

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `204 No Content`  | Success |
| `404 Not Found`   | The package or file was not found. |
