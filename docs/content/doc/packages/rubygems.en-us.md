---
date: "2021-07-20T00:00:00+00:00"
title: "RubyGems Packages Repository"
slug: "usage/packages/rubygems"
weight: 110
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "RubyGems"
    weight: 110
    identifier: "rubygems"
---

# RubyGems Packages Repository

Publish [RubyGems](https://guides.rubygems.org/) packages for your user or organization.

**Table of Contents**

{{< toc >}}

## Requirements

To work with the RubyGems package registry, you need to use the [gem](https://guides.rubygems.org/command-reference/) command line tool to consume and publish packages.

## Configuring the package registry

To register the package registry edit the `~/.gem/credentials` file and add:

```ini
---
https://gitea.example.com/api/packages/{owner}/rubygems: Bearer {token}
```

| Parameter     | Description |
| ------------- | ----------- |
| `owner`       | The owner of the package. |
| `token`       | Your [personal access token]({{< relref "doc/development/api-usage.en-us.md#authentication" >}}). |

For example:

```
---
https://gitea.example.com/api/packages/testuser/rubygems: Bearer 3bd626f84b01cd26b873931eace1e430a5773cc4
```

## Publish a package

Publish a package by running the following command:

```shell
gem push --host {host} {package_file}
```

| Parameter      | Description |
| -------------- | ----------- |
| `host`         | URL to the package registry. |
| `package_file` | Path to the package `.gem` file. |

For example:

```shell
gem push --host https://gitea.example.com/api/packages/testuser/rubygems test_package-1.0.0.gem
```

You cannot publish a package if a package of the same name and version already exists. You must delete the existing package first.

## Install a package

To install a package from the package registry you can use [Bundler](https://bundler.io) or `gem`.

### Bundler

Add a new `source` block to your `Gemfile`:

```
source "https://gitea.example.com/api/packages/{owner}/rubygems" do
  gem "{package_name}"
end
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |

For example:

```
source "https://gitea.example.com/api/packages/testuser/rubygems" do
  gem "test_package"
end
```

Afterwards run the following command:

```shell
bundle install
```

### gem

Execute the following command:

```shell
gem install --host https://gitea.example.com/api/packages/{owner}/rubygems {package_name}
```

| Parameter         | Description |
| ----------------- | ----------- |
| `owner`           | The owner of the package. |
| `package_name`    | The package name. |

For example:

```shell
gem install --host https://gitea.example.com/api/packages/testuser/rubygems test_package
```

## Supported commands

```
gem install
bundle install
gem push
```
