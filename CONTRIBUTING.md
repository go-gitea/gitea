# Contribution Guidelines

## Table of Contents

- [Contribution Guidelines](#contribution-guidelines)
  - [Introduction](#introduction)
  - [Bug reports](#bug-reports)
  - [Discuss your design](#discuss-your-design)
  - [Testing redux](#testing-redux)
  - [Vendoring](#vendoring)
  - [Translation](#translation)
  - [Code review](#code-review)
  - [Styleguide](#styleguide)
  - [Sign your work](#sign-your-work)
  - [Release Cycle](#release-cycle)
  - [Maintainers](#maintainers)
  - [Owners](#owners)
  - [Versions](#versions)
  - [Copyright](#copyright)

## Introduction

This document explains how to contribute changes to the Gitea project.
It assumes you have followed the
[installation instructions](https://docs.gitea.io/en-us/).
Sensitive security-related issues should be reported to
[security@gitea.io](mailto:security@gitea.io).

For configuring IDE or code editor to develop Gitea see [IDE and code editor configuration](contrib/ide/)

## Bug reports

Please search the issues on the issue tracker with a variety of keywords
to ensure your bug is not already reported.

If unique, [open an issue](https://github.com/go-gitea/gitea/issues/new)
and answer the questions so we can understand and reproduce the
problematic behavior.

To show us that the issue you are having is in Gitea itself, please
write clear, concise instructions so we can reproduce the behavior
(even if it seems obvious). The more detailed and specific you are,
the faster we can fix the issue. Check out [How to Report Bugs
Effectively](http://www.chiark.greenend.org.uk/~sgtatham/bugs.html).

Please be kind, remember that Gitea comes at no cost to you, and you're
getting free help.

## Discuss your design

The project welcomes submissions but please let everyone know what
you're working on if you want to change or add something to the Gitea
repositories.

Before starting to write something new for the Gitea project, please [file
an issue](https://github.com/go-gitea/gitea/issues/new). Significant
changes must go through the [change proposal
process](https://github.com/go-gitea/proposals) before they can be
accepted.

This process gives everyone a chance to validate the design, helps
prevent duplication of effort, and ensures that the idea fits inside
the goals for the project and tools. It also checks that the design is
sound before code is written; the code review tool is not the place for
high-level discussions.

## Testing redux

Before sending code out for review, run all the tests for the
whole tree to make sure the changes don't break other usage
and keep the compatibility on upgrade. To make sure you are
running the test suite exactly like we do, you should install
the CLI for [Drone CI](https://github.com/drone/drone), as
we are using the server for continous testing, following [these
instructions](http://readme.drone.io/usage/getting-started-cli). After
that you can simply call `drone exec` within your working directory and
it will try to run the test suite locally.

## Vendoring

We keep a cached copy of dependencies within the `vendor/` directory,
managing updates via [govendor](http://github.com/kardianos/govendor).

Pull requests should only include `vendor/` updates if they are part of
the same change, be it a bugfix or a feature addition.

The `vendor/` update needs to be justified as part of the PR description,
and must be verified by the reviewers and/or merger to always reference
an existing upstream commit.

## Translation

We do all translation work inside [Crowdin](https://crowdin.com/project/gitea).
The only translation that is maintained in this git repository is
[`en_US.ini`](https://github.com/go-gitea/gitea/blob/master/options/locale/locale_en-US.ini)
and is synced regularily to Crowdin. Once a translation has reached
A SATISFACTORY PERCENTAGE it will be synced back into this repo and
included in the next released version.

## Code review

Changes to Gitea must be reviewed before they are accepted, no matter who
makes the change even if it is an owner or a maintainer. We use GitHub's
pull request workflow to do that and we also use [LGTM](http://lgtm.co)
to ensure every PR is reviewed by at least 2 maintainers.

Please try to make your pull request easy to review for us.
Please read the [How to get faster PR reviews](https://github.com/kubernetes/community/blob/master/contributors/devel/faster_reviews.md) guide,
it has lots of useful tips for any project you may want to contribute.
Some of the key points:

* Make small pull requests. The smaller, the faster to review and the
  more likely it will be merged soon.
* Don't make changes unrelated to your PR. Maybe there are typos on
  some comments, maybe refactoring would be welcome on a function... but
  if that is not related to your PR, please make *another* PR for that.
* Split big pull requests into multiple small ones. An incremental change
  will be faster to review than a huge PR.

## Styleguide

For imports you should use the following format (_without_ the comments)
```go
import (
  // stdlib
  "encoding/json"
  "fmt"

  // local packages
  "code.gitea.io/gitea/models"
  "code.gitea.io/sdk/gitea"

  // external packages
  "github.com/foo/bar"
  "gopkg.io/baz.v1"
)
```

## Sign your work

The sign-off is a simple line at the end of the explanation for the
patch. Your signature certifies that you wrote the patch or otherwise
have the right to pass it on as an open-source patch. The rules are
pretty simple: If you can certify [DCO](DCO), then you just add a line
to every git commit message:

```
Signed-off-by: Joe Smith <joe.smith@email.com>
```

Please use your real name, we really dislike pseudonyms or anonymous
contributions. We are in the open-source world without secrets. If you
set your `user.name` and `user.email` git configs, you can sign your
commit automatically with `git commit -s`.

## Release Cycle

We adopted a release schedule to streamline the process of working
on, finishing, and issuing releases. The overall goal is to make a
minor release every two months, which breaks down into one month of
general development followed by one month of testing and polishing
known as the release freeze. All the feature pull requests should be
merged in the first month of one release period and during the frozen
period a corresponding release branch is open for fix backported from
master. Release candidate are made along this period for user testing to
obtain a final version that is maintained in this branch. A release is
maintained by issuing patch releases to only correct critical problems
such as crashes or security issues.

The current release cycle is aligned to start on December 25 to February
24, next is February 25 to April 24, and etc. On this cycle, we also
maybe publish the previous release minor version. For example, the
current release version is v1.1, but we maybe also publish v1.0.2. When
we publish v1.2, then we will stop publish v1.0.3.

## Maintainers

To make sure every PR is checked, we have [team
maintainers](MAINTAINERS). Every PR **MUST** be reviewed by at least
two maintainers (or owners) before it can get merged. A maintainer
should be a contributor of Gitea (or Gogs) and contributed at least
4 accepted PRs. A contributor should apply as a maintainer in the
[Discord](https://discord.gg/NsatcWJ) #develop channel. The owners
or the team maintainers may invite the contributor. A maintainer
should spend some time on code reviews. If a maintainer has no
time to do that, they should apply to leave the maintainers team
and we will give them the honor of being a member of the [advisors
team](https://github.com/orgs/go-gitea/teams/advisors). Of course, if
an advisor has time to code review, we will gladly welcome them back
to the maintainers team. If a maintainer is inactive for more than 3
months and forgets to leave the maintainers team, the owners may move
him or her from the maintainers team to the advisors team.

## Owners

Since Gitea is a pure community organization without any company support,
to keep the development healthy we will elect three owners every year. All
contributors may vote to elect up to three candidates, one of which will
be the main owner, and the other two the assistant owners. When the new
owners have been elected, the old owners will give up ownership to the
newly elected owners. If an owner is unable to do so, the other owners
will assist in ceding ownership to the newly elected owners.

After the election, the new owners should proactively agree
with our [CONTRIBUTING](CONTRIBUTING.md) requirements in the
[Discord](https://discord.gg/NsatcWJ) #general channel. Below are the
words to speak:

```
I'm honored to having been elected an owner of Gitea, I agree with
[CONTRIBUTING](CONTRIBUTING.md). I will spend part of my time on Gitea
and lead the development of Gitea.
```

To honor the past owners, here's the history of the owners and the time
they served:

* 2016-11-04 ~ 2017-12-31
  * [Lunny Xiao](https://github.com/lunny) <xiaolunwen@gmail.com>
  * [Thomas Boerger](https://github.com/tboerger) <thomas@webhippie.de>
  * [Kim Carlbäcker](https://github.com/bkcsoft) <kim.carlbacker@gmail.com>

## Versions

Gitea has the `master` branch as a tip branch and has version branches
such as `release/v0.9`. `release/v0.9` is a release branch and we will
tag `v0.9.0` for binary download. If `v0.9.0` has bugs, we will accept
pull requests on the `release/v0.9` branch and publish a `v0.9.1` tag,
after bringing the bug fix also to the master branch.

Since the `master` branch is a tip version, if you wish to use Gitea
in production, please download the latest release tag version. All the
branches will be protected via GitHub, all the PRs to every branch must
be reviewed by two maintainers and must pass the automatic tests.

## Copyright

Code that you contribute should use the standard copyright header:

```
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
```

Files in the repository contain copyright from the year they are added
to the year they are last changed. If the copyright author is changed,
just paste the header below the old one.
