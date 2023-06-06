---
date: "2021-07-20T00:00:00+00:00"
title: "npm Package Registry"
slug: "npm"
weight: 70
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "npm"
    weight: 70
    identifier: "npm"
---

# npm Package Registry

Publish [npm](https://www.npmjs.com/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the npm package registry, you need [Node.js](https://nodejs.org/en/download/) coupled with a package manager such as [Yarn](https://classic.yarnpkg.com/en/docs/install) or [npm](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm/) itself.

The registry supports [scoped](https://docs.npmjs.com/misc/scope/) and unscoped packages.

The following examples use the `npm` tool with the scope `@test`.

## Configuring the package registry

To register the package registry you need to configure a new package source.

```shell
npm config set {scope}:registry https://gitea.example.com/api/packages/{owner}/npm/
npm config set -- '//gitea.example.com/api/packages/{owner}/npm/:_authToken' "{token}"
```

| Parameter    | Description |
| ------------ | ----------- |
| `scope`      | The scope of the packages. |
| `owner`      | The owner of the package. |
| `token`      | Your [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}). |

For example:

```shell
npm config set @test:registry https://gitea.example.com/api/packages/testuser/npm/
npm config set -- '//gitea.example.com/api/packages/testuser/npm/:_authToken' "personal_access_token"
```

or without scope:

```shell
npm config set registry https://gitea.example.com/api/packages/testuser/npm/
npm config set -- '//gitea.example.com/api/packages/testuser/npm/:_authToken' "personal_access_token"
```

## Publish a package

Publish a package by running the following command in your project:

```shell
npm publish
```

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Unpublish a package

Delete a package by running the following command:

```shell
npm unpublish {package_name}[@{package_version}]
```

| Parameter         | Description |
| ----------------- | ----------- |
| `package_name`    | The package name. |
| `package_version` | The package version. |

For example:

```shell
npm unpublish @test/test_package
npm unpublish @test/test_package@1.0.0
```

## Install a package

To install a package from the package registry, execute the following command:

```shell
npm install {package_name}
```

| Parameter      | Description |
| -------------- | ----------- |
| `package_name` | The package name. |

For example:

```shell
npm install @test/test_package
```

## Tag a package

The registry supports [version tags](https://docs.npmjs.com/adding-dist-tags-to-packages/) which can be managed by `npm dist-tag`:

```shell
npm dist-tag add {package_name}@{version} {tag}
```

| Parameter      | Description |
| -------------- | ----------- |
| `package_name` | The package name. |
| `version`      | The version of the package. |
| `tag`          | The tag name. |

For example:

```shell
npm dist-tag add test_package@1.0.2 release
```

The tag name must not be a valid version. All tag names which are parsable as a version are rejected.

## Search packages

The registry supports [searching](https://docs.npmjs.com/cli/v7/commands/npm-search/) but does not support special search qualifiers like `author:gitea`.

## Supported commands

```
npm install
npm ci
npm publish
npm unpublish
npm dist-tag
npm view
npm search
```
