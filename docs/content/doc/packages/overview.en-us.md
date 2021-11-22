---
date: "2021-07-20T00:00:00+00:00"
title: "Package Registry"
slug: "overview"
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Overview"
    weight: 1
    identifier: "overview"
---

# Package Registry

The Package Registry can be used as a public or private registry for common package managers.

These package types are supported:

- [Generic]({{< relref "doc/packages/generic.en-us.md" >}})
- [Composer]({{< relref "doc/packages/composer.en-us.md" >}})
- [NuGet]({{< relref "doc/packages/nuget.en-us.md" >}})
- [npm]({{< relref "doc/packages/npm.en-us.md" >}})
- [Maven]({{< relref "doc/packages/maven.en-us.md" >}})
- [PyPI]({{< relref "doc/packages/pypi.en-us.md" >}})
- [RubyGems]({{< relref "doc/packages/rubygems.en-us.md" >}})

**Table of Contents**

{{< toc >}}

## View packages

You can view the packages of a repository on the repository page.

1. Go to the repoistory.
1. Go to **Packages** in the navigation bar.

To view more details about a package, select the name of the package.

## Download a package

To download a package from your repository:

1. Go to **Packages** in the navigation bar.
1. Select the name of the package to view the details.
1. In the **Assets** section, select the name of the package file you want to download.

## Delete a package

You cannot edit a package after you published it in the Package Registry. Instead, you
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
