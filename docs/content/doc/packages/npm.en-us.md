---
date: "2021-07-20T00:00:00+00:00"
title: "npm Packages Repository"
slug: "npm"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "npm"
    weight: 30
    identifier: "npm"
---

# npm Packages Repository

Publish [npm](https://www.npmjs.com/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the npm package registry, you need [Node.js and npm](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm/) or an other tool like [Yarn](https://classic.yarnpkg.com/en/docs/install).

The registry supprots [scoped](https://docs.npmjs.com/misc/scope/) and unscoped packages.

The following examples use the `npm` tool with the scope `@test`.

## Configuring the package registry

To register the project’s package registry you need to configure a new package source.

```shell
npm config set {scope}:registry https://gitea.example.com/api/v1/packages/{owner}/npm/
npm config set -- '//gitea.example.com/api/v1/packages/{owner}/npm/:_authToken' "{token}"
```

| Parameter    | Description |
| ------------ | ----------- |
| `scope`      | The scope of the packages. |
| `owner`      | The owner of the package. |
| `token`      | Your [personal access token]({{< relref "doc/developers/api-usage.en-us.md#authentication" >}}). |

For example:

```shell
npm config set @test:registry https://gitea.example.com/api/v1/packages/testuser/npm/
npm config set -- '//gitea.example.com/api/v1/packages/testuser/npm/:_authToken' "personal_access_token"
```

or without scope:

```shell
npm config set registry https://gitea.example.com/api/v1/packages/testuser/npm/
npm config set -- '//gitea.example.com/api/v1/packages/testuser/npm/:_authToken' "personal_access_token"
```

## Publish a package

Publish a package by running the following command in your project:

```shell
npm publish
```

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Install a package

To install a package from the package registry, execute the following command:

```shell
npm install {scope}/{package_name}
```

| Parameter      | Description |
| -------------- | ----------- |
| `scope`        | The scope of the packages. |
| `package_name` | The package name. |

For example:

```shell
npm install @test/test_package
```