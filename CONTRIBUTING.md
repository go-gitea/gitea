# Contribution Guidelines

## Introduction

This document explains how to contribute changes to the Gitea project.
It assumes you have followed the
[installation instructions](https://github.com/go-gitea/docs/tree/master/en-US/installation)

Sensitive security-related issues should be reported to [security@gitea.io](mailto:security@gitea.io).

## Bug reports

Please search the issues on the issue tracker with a variety of keywords to
ensure your bug is not already reported.

If unique, [open an issue](https://github.com/go-gitea/gitea/issues/new)
and answer the questions so we can understand and reproduce the problematic
behavior.

The burden is on you to convince us that it is actually a bug in Gitea. This
is easiest to do when you write clear, concise instructions so we can reproduce
the behavior (even if it seems obvious). The more detailed and specific you are,
the faster we will be able to help you. Check out
[How to Report Bugs Effectively](http://www.chiark.greenend.org.uk/~sgtatham/bugs.html).

Please be kind, remember that Gitea comes at no cost to you, and you're
getting free help.

## Discuss your design

The project welcomes submissions but please let everyone know what
you're working on if you want to change or add something to the Gitea repositories.

Before starting to write something new for the Gitea project,
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
tree to make sure the changes don't break other usage and keep the compatibility on upgrade:

After running for a while, the command should print

```
ALL TESTS PASSED
```

## Code review

Changes to Gitea must be reviewed before they are accepted,
no matter who makes the change even if an owners or a maintainer.
We use github's pull request workflow to do that and use [lgtm](http://lgtm.co) to ensure every PR is reviewed by at least 2 maintainers.

## Sign your work

The sign-off is a simple line at the end of the explanation for the patch. Your
signature certifies that you wrote the patch or otherwise have the right to pass
it on as an open-source patch. The rules are pretty simple: If you can certify
[DCO](DCO), then you just add a line to every git commit message:

```
Signed-off-by: Joe Smith <joe.smith@email.com>
```

Please use your real name, we really dislike pseudonyms or anonymous
contributions. We are in the opensource world without secrets. If you set your
`user.name` and `user.email` git configs, you can sign your commit automatically
with `git commit -s`.

## Contributors

Everyone who sent a PR to Gitea that gets accepted will be as a contributor. Please send a PR to add your name to
[CONTRIBUTORS](CONTRIBUTORS). For the format, see the [CONTRIBUTORS](CONTRIBUTORS).

## Maintainers

To make sure every PR have been checked, we make a team maintainers. Any PR MUST be reviewed and by at least two maintainers before it can get merged.
Maintainers should be a contributor of gitea(or gogs) and contributed at least 4 accepted PRs. And a contributor should apply as a maintainer in [gitter Gitea develop](https://gitter.im/go-gitea/develop).
And the owners or the team maintainer could invite the contributor. A maintainer should spend some time on code reviews. If some maintainer have no time
to do that, he should apply to leave maintainers team and we will give him an honor to be as a member of advisor team. Of course, if an advisor have time to code view, welcome it back to maintainers team.
If some one have no time to code view and forget to leave the maintainers, the owners have the power to move him from maintainers team to advisors team.

## Owners

Since Gitea is a pure community organization without any company support, to keep the development healthly We will elect the owners every year. Every time we will elect three owners.
All the contributers could vote for three owners, one is the main owner, the other two are assistant owners. When the new owners have been elected, the old owners MUST move the power to the new owners. 
If some owner don't obey these rules, the other owners are allowed to revoke his owner status.

After the election, the new owners should say he agrees with these rules on the [CONTRIBUTING](CONTRIBUTING.md) on the [Gitter Gitea Channel](https://gitter.im/go-gitea/gitea). Below is the word to speak

```
I'm glad to be an owner of Gitea, I agree with [CONTRIBUTING](CONTRIBUTING.md). I will spend part of my time on gitea and lead the development of gitea.
```

For a honor to the owners, this document will add the history owners below:

2016-11-04 ~ 2017-12-31 lunny <xiaolunwen@gmail.com> tboerger <thomas@webhippie.de> bkcsoft <kim.carlbacker@gmail.com>

## Versions

Gitea has one master as a tip branch and have many version branch such as v0.9. v0.9 is a release branch and we will tag v0.9.0 both for binary download.
If v0.9.0 have some bugs, we will accept PR on v0.9 and publish v0.9.1 and merge bug PR to master.

Branch master is a tip version, so if you wish a production usage, please download the latest release tag version. All the branch will be protected via github,
All the PRs to all the branches should be review by two maintainers and pass the automatic tests.

## Copyright

Code that you contribute should use the standard copyright header:

```
// Copyright 2016 - 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
```

Files in the repository are copyright the year they are added and the year they are last changed. If the copyright author is changed, just copy the head
below the old one.
