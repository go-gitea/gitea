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

You can use labels to classify issues and pull requests and to improve your overview over them.

## Creating Labels

Labels can be created for a repository by going to `Issues` and clicking on `Labels`. \
For organizations, you can define organization-wide labels that are shared with all organization repositories, including both already-existing repositories as well as newly created ones. \
Organization-wide labels can be created in the organization `Settings`.

Labels have a mandatory name, a mandatory color, an optional description, and must either be exclusive or not (see `Scoped labels` below).

When you create a repo, you can ensure certain labels exist by using the `Issue Labels` option. \
This option lists a number of available label sets that are [configured globally on your instance](../customizing-gitea/#labels). \
Its contained labels will all be created as well while creating the repo.

## Scoped Labels

You can decrease your error susceptibility by using scoped labels. \
A scoped label is a label that is marked as `exclusive` and contains `/` in its name (not at either end of the name). \
For example, if label `A` is called `scope/item`, label `B` is called `scope/second-item`, and both are marked as `exclusive`, an issue cannot be labeled with both `A` and `B` at the same time. \
Issues can have at most one of these labels per scope, or none. \
The scope of a label is determined based on the **last** `/`, so for example the scope of label `subscope/subscope2/item` would be `subscope/subscope2`.

## Filtering by Label

Issue and pull request lists can be filtered by label. Selecting multiple labels shows issues and pull requests that have all selected labels assigned.

By holding alt to click the label, issues and pull requests with the chosen label are excluded from the list.
