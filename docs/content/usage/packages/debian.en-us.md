---
date: "2023-01-07T00:00:00+00:00"
title: "Debian Package Registry"
slug: "debian"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Debian"
    sidebar_position: 35
    identifier: "debian"
---

# Debian Package Registry

Publish [Debian](https://www.debian.org/distrib/packages) packages for your user or organization.

## Requirements

To work with the Debian registry, you need to use a HTTP client like `curl` to upload and a package manager like `apt` to consume packages.

The following examples use `apt`.

## Configuring the package registry

To register the Debian registry add the url to the list of known apt sources:

```shell
echo "deb https://gitea.example.com/api/packages/{owner}/debian {distribution} {component}" | sudo tee -a /etc/apt/sources.list.d/gitea.list
```

| Placeholder    | Description |
| -------------- | ----------- |
| `owner`        | The owner of the package. |
| `distribution` | The distribution to use. |
| `component`    | The component to use. |

If the registry is private, provide credentials in the url. You can use a password or a [personal access token](development/api-usage.md#authentication):

```shell
echo "deb https://{username}:{your_password_or_token}@gitea.example.com/api/packages/{owner}/debian {distribution} {component}" | sudo tee -a /etc/apt/sources.list.d/gitea.list
```

The Debian registry files are signed with a PGP key which must be known to apt:

```shell
sudo curl https://gitea.example.com/api/packages/{owner}/debian/repository.key -o /etc/apt/trusted.gpg.d/gitea-{owner}.asc
```

Afterwards update the local package index:

```shell
apt update
```

## Publish a package

To publish a Debian package (`*.deb`), perform a HTTP `PUT` operation with the package content in the request body.

```
PUT https://gitea.example.com/api/packages/{owner}/debian/pool/{distribution}/{component}/upload
```

| Parameter      | Description |
| -------------- | ----------- |
| `owner`        | The owner of the package. |
| `distribution` | The distribution may match the release name of the OS, ex: `bionic`. |
| `component`    | The component can be used to group packages or just `main` or similar. |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.deb \
     https://gitea.example.com/api/packages/testuser/debian/pool/bionic/main/upload
```

If you are using 2FA or OAuth use a [personal access token](development/api-usage.md#authentication) instead of the password.
You cannot publish a file with the same name twice to a package. You must delete the existing package version first.

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `201 Created`     | The package has been published. |
| `400 Bad Request` | The package name, version, distribution, component or architecture are invalid. |
| `409 Conflict`    | A package file with the same combination of parameters exists already. |

## Delete a package

To delete a Debian package perform a HTTP `DELETE` operation. This will delete the package version too if there is no file left.

```
DELETE https://gitea.example.com/api/packages/{owner}/debian/pool/{distribution}/{component}/{package_name}/{package_version}/{architecture}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |
| `package_version` | The package version. |
| `distribution`    | The package distribution. |
| `component`       | The package component. |
| `architecture`    | The package architecture. |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/debian/pools/bionic/main/test-package/1.0.0/amd64
```

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `204 No Content`  | Success |
| `404 Not Found`   | The package or file was not found. |

## Install a package

To install a package from the Debian registry, execute the following commands:

```shell
# use latest version
apt install {package_name}
# use specific version
apt install {package_name}={package_version}
```
