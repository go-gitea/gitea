---
date: "2023-01-07T00:00:00+00:00"
title: "Debian Packages Repository"
slug: "packages/debian"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Debian"
    weight: 45
    identifier: "debian"
---

# Debian Packages Repository

Publish Debian packages to a APT repository for your user or organization.

**Table of Contents**

{{< toc >}}

## Authenticate to the package registry

To authenticate to the Package Registry, you need to provide [custom HTTP headers or use HTTP Basic authentication]({{< relref "doc/developers/api-usage.en-us.md#authentication" >}}).

## Publish a package

To publish a Debian package, perform a HTTP PUT operation with the package content in the request body.
You cannot publish a file with the same name twice to a package. You must delete the existing package version first.

```
PUT https://gitea.example.com/api/packages/{owner}/debian/files/{package_name}/{package_version}/{package_architecture}
```

| Parameter              | Description |
| ---------------------  | ----------- |
| `owner`                | The owner of the package. |
| `package_name`         | The package name. It can contain only lowercase letters (`a-z`), uppercase letter (`A-Z`), numbers (`0-9`), dots (`.`), hyphens (`-`), pluses (`+`), or underscores (`_`). |
| `package_version`      | The package version, a non-empty string without trailing or leading whitespaces. |
| `package_architecture` | The package architecture. Can be a Debian machine architecture as described in [Debian Architecture specifications](https://www.debian.org/doc/debian-policy/ch-customized-programs.html#s-arch-spec), `all`, or `source`. |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/file.deb \
     https://gitea.example.com/api/packages/testuser/debian/files/test-package/1.0.0/amd64
```

If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/developers/api-usage.en-us.md#authentication" >}}) instead of the password.

The server reponds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `201 Created`     | The package has been published. |
| `400 Bad Request` | The package name and/or version and/or file name are invalid. |
| `409 Conflict`    | A file with the same name exist already in the package. |

## Delete a package file

To delete a file of a generic package perform a HTTP DELETE operation. This will delete the package version too if there is no file left.

```
DELETE https://gitea.example.com/api/packages/{owner}/debian/files/{package_name}/{package_version}/{package_architecture}
```

| Parameter              | Description |
| ---------------------- | ----------- |
| `owner`                | The owner of the package. |
| `package_name`         | The package name. |
| `package_version`      | The package version. |
| `package_architecture` | The package architecture. |

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_token_or_password -X DELETE \
     https://gitea.example.com/api/packages/testuser/debian/files/test-package/1.0.0/amd64
```

The server reponds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `204 No Content`  | Success |
| `404 Not Found`   | The package or file was not found. |

## Download a package

To download a generic package perform a HTTP GET operation.

```
GET https://gitea.example.com/api/packages/{owner}/debian/pool/{package_name}_{package_version}_{package_architecture}.deb
```

| Parameter              | Description |
| ---------------------- | ----------- |
| `owner`                | The owner of the package. |
| `package_name`         | The package name. |
| `package_version`      | The package version. |
| `package_architecture` | The package architecture. |

The file content is served in the response body. The response content type is `application/octet-stream`.

Example request using HTTP Basic authentication:

```shell
curl https://gitea.example.com/api/packages/testuser/debian/pool/test-package_1.0.0_amd64.deb
```

The server reponds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `200 OK`          | Success |
| `404 Not Found`   | The package or file was not found. |

## Using APT to download and install packages

### Generating a PGP key pair

The APT repository needs a PGP keypair to sign the Release files. With openpgp installed, generate a keypair with the following:

```shell
gpg --full-gen-key
```

### Adding signing keys to Gitea Debian repository

The private and public keys can be exported and should be placed in Gitea's data directory:

```shell
gpg --export-secret-key <KEYID> > <gitea_data>/debian.gpg
gpg --export --armor <KEYID> > <gitea_data>/debian_public.gpg
```

Once the keys have been added, the Release and Packages files are generated after upload or deletion of a package file.

### Adding the key and repository to APT

To add the key from the server to the APT keyring:

```shell
curl https://gitea.example.com/api/packages/test-user/debian/debian.key \
  | sudo tee /etc/apt/trusted.gpg.d/gitea-repo.asc
```

The URL of the repository is: `https://<your_gitea_address>/api/packages/<user>/debian`.  
The "distro" portion should be `gitea` and so far, only `main` component is supported.

```shell
echo "deb https://gitea.example.com/api/packages/test-user/debian gitea main" \
  | sudo tee /etc/apt/sources.list.d/gitea.list
sudo apt update
```
