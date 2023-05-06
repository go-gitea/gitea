---
date: "2021-07-20T00:00:00+00:00"
title: "Package Registry"
slug: "overview"
weight: 1
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Overview"
    weight: 1
    identifier: "packages-overview"
---

# Package Registry

Starting with Gitea **1.17**, the Package Registry can be used as a public or private registry for common package managers.

**Table of Contents**

{{< toc >}}

## Supported package managers

The following package managers are currently supported:

| Name | Language | Package client |
| ---- | -------- | -------------- |
| [Alpine]({{< relref "doc/usage/packages/alpine.en-us.md" >}}) | - | `apk` |
| [Cargo]({{< relref "doc/usage/packages/cargo.en-us.md" >}}) | Rust | `cargo` |
| [Chef]({{< relref "doc/usage/packages/chef.en-us.md" >}}) | - | `knife` |
| [Composer]({{< relref "doc/usage/packages/composer.en-us.md" >}}) | PHP | `composer` |
| [Conan]({{< relref "doc/usage/packages/conan.en-us.md" >}}) | C++ | `conan` |
| [Conda]({{< relref "doc/usage/packages/conda.en-us.md" >}}) | - | `conda` |
| [Container]({{< relref "doc/usage/packages/container.en-us.md" >}}) | - | any OCI compliant client |
| [Debian]({{< relref "doc/usage/packages/debian.en-us.md" >}}) | - | `apt` |
| [Generic]({{< relref "doc/usage/packages/generic.en-us.md" >}}) | - | any HTTP client |
| [Helm]({{< relref "doc/usage/packages/helm.en-us.md" >}}) | - | any HTTP client, `cm-push` |
| [Maven]({{< relref "doc/usage/packages/maven.en-us.md" >}}) | Java | `mvn`, `gradle` |
| [npm]({{< relref "doc/usage/packages/npm.en-us.md" >}}) | JavaScript | `npm`, `yarn`, `pnpm` |
| [NuGet]({{< relref "doc/usage/packages/nuget.en-us.md" >}}) | .NET | `nuget` |
| [Pub]({{< relref "doc/usage/packages/pub.en-us.md" >}}) | Dart | `dart`, `flutter` |
| [PyPI]({{< relref "doc/usage/packages/pypi.en-us.md" >}}) | Python | `pip`, `twine` |
| [RPM]({{< relref "doc/usage/packages/rpm.en-us.md" >}}) | - | `yum`, `dnf` |
| [RubyGems]({{< relref "doc/usage/packages/rubygems.en-us.md" >}}) | Ruby | `gem`, `Bundler` |
| [Swift]({{< relref "doc/usage/packages/rubygems.en-us.md" >}}) | Swift | `swift` |
| [Vagrant]({{< relref "doc/usage/packages/vagrant.en-us.md" >}}) | - | `vagrant` |

**The following paragraphs only apply if Packages are not globally disabled!**

## Repository-Packages

A package always belongs to an owner (a user or organisation), not a repository.
To link an (already uploaded) package to a repository, open the settings page
on that package and choose a repository to link this package to.
The entire package will be linked, not just a single version.

Linking a package results in showing that package in the repository's package list,
and shows a link to the repository on the package site (as well as a link to the repository issues).

## Access Restrictions

| Package owner type | User | Organization |
|--------------------|------|--------------|
| **read** access    | public, if user is public too; otherwise for this user only | public, if org is public, otherwise for org members only |
| **write** access   | owner only | org members with admin or write access to the org |

N.B.: These access restrictions are [subject to change](https://github.com/go-gitea/gitea/issues/19270), where more finegrained control will be added via a dedicated organization team permission.

## Create or upload a package

Depending on the type of package, use the respective package-manager for that. Check out the sub-page of a specific package manager for instructions.

## View packages

You can view the packages of a repository on the repository page.

1. Go to the repository.
1. Go to **Packages** in the navigation bar.

To view more details about a package, select the name of the package.

## Download a package

To download a package from your repository:

1. Go to **Packages** in the navigation bar.
1. Select the name of the package to view the details.
1. In the **Assets** section, select the name of the package file you want to download.

## Delete a package

You cannot edit a package after you have published it in the Package Registry. Instead, you
must delete and recreate it.

To delete a package from your repository:

1. Go to **Packages** in the navigation bar.
1. Select the name of the package to view the details.
1. Click **Delete package** to permanently delete the package.

## Disable the Package Registry

The Package Registry is automatically enabled. To disable it for a single repository:

1. Go to **Settings** in the navigation bar.
1. Disable **Enable Repository Packages Registry**.

Previously published packages are not deleted by disabling the Package Registry.
