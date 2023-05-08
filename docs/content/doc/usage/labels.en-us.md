---
date: "2023-03-04T19:00:00+00:00"
title: "Labels"
slug: "labels"
weight: 13
toc: false
draft: false
aliases:
  - /en-us/labels
menu:
  sidebar:
    parent: "usage"
    name: "Labels"
    weight: 13
    identifier: "labels"
---

# Labels

You can use labels to classify issues and pull requests and to improve your overview over them.

## Creating Labels

For repositories, labels can be created by going to `Issues` and clicking on `Labels`.

For organizations, you can define organization-wide labels that are shared with all organization repositories, including both already-existing repositories as well as newly created ones. Organization-wide labels can be created in the organization `Settings`.

Labels have a mandatory name, a mandatory color, an optional description, and must either be exclusive or not (see `Scoped Labels` below).

When you create a repository, you can ensure certain labels exist by using the `Issue Labels` option. This option lists a number of available label sets that are [configured globally on your instance](../customizing-gitea/#labels). Its contained labels will all be created as well while creating the repository.

## Scoped Labels

Scoped labels are used to ensure at most a single label with the same scope is assigned to an issue or pull request. For example, if labels `kind/bug` and `kind/enhancement` have the Exclusive option set, an issue can only be classified as a bug or an enhancement.

A scoped label must contain `/` in its name (not at either end of the name). The scope of a label is determined based on the **last** `/`, so for example the scope of label `scope/subscope/item` is `scope/subscope`.

## Filtering by Label

Issue and pull request lists can be filtered by label. Selecting multiple labels shows issues and pull requests that have all selected labels assigned.

By holding alt to click the label, issues and pull requests with the chosen label are excluded from the list.
