---
date: "2023-01-10T00:00:00+00:00"
title: "Swift Packages Repository"
slug: "usage/packages/swift"
weight: 95
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Swift"
    weight: 95
    identifier: "swift"
---

# Swift Packages Repository

Publish [Swift](hhttps://www.swift.org/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the Swift package registry, you need to use [swift](https://www.swift.org/getting-started/) to consume and a HTTP client (like `curl`) to publish packages.

## Configuring the package registry

To register the package registry and provide credentials, execute:

```shell
swift package-registry set https://gitea.example.com/api/packages/{owner}/swift -login {username} -password {password}
```

| Placeholder  | Description |
| ------------ | ----------- |
| `owner`      | The owner of the package. |
| `username`   | Your Gitea username. |
| `password`   | Your Gitea password. If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) instead of the password. |

The login is optional and only needed if the package registry is private.

## Publish a package

First you have to pack the contents of your package:

```shell
swift package archive-source
```

To publish the package perform a HTTP PUT request with the package content in the request body.

```shell --user your_username:your_password_or_token \
curl -X PUT --user {username}:{password} \
	 -H "Accept: application/vnd.swift.registry.v1+json" \
	 -F source-archive=@/path/to/package.zip \
	 -F metadata={metadata} \
	 https://gitea.example.com/api/packages/{owner}/swift/{scope}/{name}/{version}
```

| Placeholder | Description |
| ----------- | ----------- |
| `username`  | Your Gitea username. |
| `password`  | Your Gitea password. If you are using 2FA or OAuth use a [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}) instead of the password. |
| `owner`     | The owner of the package. |
| `scope`     | The package scope. |
| `name`      | The package name. |
| `version`   | The package version. |
| `metadata`  | (Optional) The metadata of the package. JSON encoded subset of https://schema.org/SoftwareSourceCode |

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Install a package

To install a Swift package from the package registry, add it in the `Package.swift` file dependencies list:

```
dependencies: [
	.package(id: "{scope}.{name}", from:"{version}")
]
```

| Parameter   | Description |
| ----------- | ----------- |
| `scope`     | The package scope. |
| `name`      | The package name. |
| `version`   | The package version. |

Afterwards execute the following command to install it:

```shell
swift package resolve
```
