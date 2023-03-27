---
date: "2021-07-20T00:00:00+00:00"
title: "Composer Packages Repository"
slug: "composer"
weight: 10
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Composer"
    weight: 10
    identifier: "composer"
---

# Composer Packages Repository

Publish [Composer](https://getcomposer.org/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the Composer package registry, you can use [Composer](https://getcomposer.org/download/) to consume and a HTTP upload client like `curl` to publish packages.

## Publish a package

To publish a Composer package perform a HTTP PUT operation with the package content in the request body.
The package content must be the zipped PHP project with the `composer.json` file.
You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

```
PUT https://gitea.example.com/api/packages/{owner}/composer
```

| Parameter  | Description |
| ---------- | ----------- |
| `owner`    | The owner of the package. |

If the `composer.json` file does not contain a `version` property, you must provide it as a query parameter:

```
PUT https://gitea.example.com/api/packages/{owner}/composer?version={x.y.z}
```

Example request using HTTP Basic authentication:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/project.zip \
     https://gitea.example.com/api/packages/testuser/composer
```

Or specify the package version as query parameter:

```shell
curl --user your_username:your_password_or_token \
     --upload-file path/to/project.zip \
     https://gitea.example.com/api/packages/testuser/composer?version=1.0.3
```

If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) instead of the password.

The server responds with the following HTTP Status codes.

| HTTP Status Code  | Meaning |
| ----------------- | ------- |
| `201 Created`     | The package has been published. |
| `400 Bad Request` | The package name and/or version are invalid or a package with the same name and version already exist. |

## Configuring the package registry

To register the package registry you need to add it to the Composer `config.json` file (which can usually be found under `<user-home-dir>/.composer/config.json`):

```json
{
  "repositories": [{
      "type": "composer",
      "url": "https://gitea.example.com/api/packages/{owner}/composer"
   }
  ]
}
```

To access the package registry using credentials, you must specify them in the `auth.json` file as follows:

```json
{
  "http-basic": {
    "gitea.example.com": {
      "username": "{username}",
      "password": "{password}"
    }
  }
}
```

| Parameter  | Description |
| ---------- | ----------- |
| `owner`    | The owner of the package. |
| `username` | Your Gitea username. |
| `password` | Your Gitea password or a personal access token. |

## Install a package

To install a package from the package registry, execute the following command:

```shell
composer require {package_name}
```

Optional you can specify the package version:

```shell
composer require {package_name}:{package_version}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `package_name`    | The package name. |
| `package_version` | The package version. |
