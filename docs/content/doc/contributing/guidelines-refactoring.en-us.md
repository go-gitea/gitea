---
date: "2023-02-14T00:00:00+00:00"
title: "Guidelines for Refactoring"
slug: "guidelines-refactoring"
weight: 20
toc: false
draft: false
aliases:
  - /en-us/guidelines-refactoring
menu:
  sidebar:
    parent: "contributing"
    name: "Guidelines for Refactoring"
    weight: 20
    identifier: "guidelines-refactoring"
---

# Guidelines for Refactoring

**Table of Contents**

{{< toc >}}

## Background

Since the first line of code was written at Feb 12, 2014, Gitea has grown to be a large project.
As a result, the codebase has become larger and larger. The larger the codebase is, the more difficult it is to maintain.
A lot of outdated mechanisms exist, a lot of frameworks are mixed together, some legacy code might cause bugs and block new features.
To make the codebase more maintainable and make Gitea better, developers should keep in mind to use modern mechanisms to refactor the old code.

This document is a collection of guidelines for refactoring the codebase.

## Refactoring Suggestions

* Design more about the future, but not only resolve the current problem.
* Reduce ambiguity, reduce conflicts, improve maintainability.
* Describe the refactoring, for example:
  * Why the refactoring is needed.
  * How the legacy problems would be solved.
  * What's the Pros/Cons of the refactoring.
* Only do necessary changes, keep the old logic as much as possible.
* Introduce some intermediate steps to make the refactoring easier to review, a complete refactoring plan could be done in several PRs.
* If there is any divergence, the TOC(Technical Oversight Committee) should be involved to help to make decisions.
* Add necessary tests to make sure the refactoring is correct.
* Non-bug refactoring is preferred to be done at the beginning of a milestone, it would be easier to find problems before the release.

## Reviewing & Merging Suggestions

* A refactoring PR shouldn't be kept open for a long time (usually 7 days), it should be reviewed as soon as possible.
* A refactoring PR should be merged as soon as possible, it should not be blocked by other PRs.
* If there is no objection from TOC, a refactoring PR could be merged after 7 days with one core member's approval (not the author).
* Tolerate some dirty/hacky intermediate steps if the final result is good.
* Tolerate some regression bugs if the refactoring is necessary, fix bugs as soon as possible.
