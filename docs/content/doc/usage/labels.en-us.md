---
date: "2023-03-04T19:00:00+00:00"
title: "Usage: Labels"
slug: "labels"
weight: 13
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Labels"
    weight: 13
    identifier: "labels"
---

# Labels

Issues and pull requests can be assigned labels for organization.

## Creating Labels

Labels can be created for a repository by going to Issues and choosing Labels. For organizations, labels available to all repositories can be created in the organization Settings.

Labels have a name, color and optional description.

If no labels exist a [default label set](../customizing-gitea/#labels) is suggested.

## Scoped Labels

Labels can be grouped using scopes indicated with a `/` separator in the name. When a label is named for example `scope/item`, the label display will changed to show the scope and item separately.

Mutually exclusive labels can be created by enabling the Exclusive option for scoped labels. This makes it so only one label in the same scope can be assigned to an issue or pull request.

## Filtering by Label

Issue and pull request lists can be filtered by label. Selecting multiple labels shows issues and pull requests that have all selected labels assigned.

By holding alt to click the label, issues and pull requests with the chosen label are excluded from the list.
