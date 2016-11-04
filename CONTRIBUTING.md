# Contribution Guidelines

## Introduction

This document explains how to contribute changes to the Gitea project.
It assumes you have followed the
[installation instructions](https://github.com/go-gitea/docs/tree/master/en-US/installation)

Sensitive security-related issues should be reported to [security@gitea.io](mailto:security@gitea.io).

## Discuss your design

The project welcomes submissions but please let everyone know what
you're working on if you want to change or add to the Gitea repositories.

Before undertaking to write something new for the Gitea project,
please [file an issue](https://github.com/go-gitea/gitea/issues/new).
Significant changes must go through the
[change proposal process](https://github.com/go-gitea/proposals)
before they can be accepted.

This process gives everyone a chance to validate the design,
helps prevent duplication of effort,
and ensures that the idea fits inside the goals for the project and tools.
It also checks that the design is sound before code is written;
the code review tool is not the place for high-level discussions.

## Testing redux

Before sending code out for review, run all the tests for the whole
tree to make sure the changes don't break other usage and keep the compitable when upgrade:

After running for a while, the command should print

```
ALL TESTS PASSED
```

## Code review

Changes to Gitea must be reviewed before they are accepted,
no matter who makes the change even if you are the owners or maintainers.
We use github's pull request workflow to do that and use lgtm to keep every PR has more than 2 maintainers to reviewed.

## Contributers

Everyone who sent a PR to gitea(or gogs) and accepted will be as a contributor. Please send a PR to add your name on
[CONTRIBUTORS](CONTRIBUTORS) and write PR numbers on the PR comment. For the format, see the [CONTRIBUTORS](CONTRIBUTORS).

## Maintainers

To keep every PR have been checked, we make a team maintainers. Any PR(include owners' PR) MUST be reviewed and by other two maintainers to check before merged.
Maintainers should be a contributor of gitea(or gogs) and contributed more than 4 PRs(included). And a contributar should apply as a maintainer in [gitter gitea develop](https://gitter.im/go-gitea/develop).
And the owners or the maintainers team maintainer could invite the contributor. A maintainer should spend some time on code view PRs. If some maintainer have no time
to do that, he should apply to leave maintainers team and we will give him an honor to be as a member of advisor team. Of course, if an advisor have time to code view, welcome it back to maintainers team.
If some one have no time to code view and forget to leave the maintainers, the owners have the power to move him from maintainers team to advisors team.

## Owners

Since gitea is a pure community organization with no any company support now, to keep it development healthly We will elect the owners every year. Every time we will elect three owners.
All the contributers could vote for three owners, one is the main owner, the other two are assistant owners. When the new owners have been elected, the old owners MUST move the power to the new owners. 
If someone owners don't obey this CONTRIBUTING, all the contributors could fork a new project and continue the project. 

After the election, the new owners should say he agree with the CONTRIBUTING on the [Gitter Gitea Channel](https://gitter.im/go-gitea/gitea). Below is the word to speak

```
I'm glad to be as an owner of gitea, I agree with [CONTRIBUTING](CONTRIBUTING.md). I will spend part of my time on gitea and lead the development of gitea.
```

For a honor to the owners, this document will add the history owners below:

2016 - 2017 lunny<xiaolunwen@gmail.com> tboerger<thomas@webhippie.de> bkcsoft<kim.carlbacker@gmail.com>

## Versions

Gitea has one master as a tip branch and have many version branch such as v0.9. v0.9 is a release branch and we will tag v0.9.0 both for binary download.
If v0.9.0 have some bugs, we will accept PR on v0.9 and publish v0.9.1 and merge bug PR to master.

Branch master is a tip version, so if you wish a production usage, please download the latest release tag version. All the branch will be protected via github,
All the PRs to all the branches should be review by two maintainers and pass the automatic tests.

## Copyright

Code that you contribute should use the standard copyright header:

```
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
```

Files in the repository are copyright the year they are added. It is not
necessary to update the copyright year on files that you change.
