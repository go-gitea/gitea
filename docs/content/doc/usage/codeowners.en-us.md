---
date: "2023-06-02T16:00:00+00:00"
title: "Code Owners"
slug: "codeowners"
weight: 30
toc: false
draft: false
aliases:
  - /en-us/codeowners
menu:
  sidebar:
    parent: "usage"
    name: "Code Owners"
    weight: 30
    identifier: "codeowners"
---

# Code Owners
You can use a CODEOWNERS file to define individuals or teams that are responsible for code in a repository. Code owners are automatically requested for review when someone opens a pull request that modifies code that they own.

If a file has a code owner, you can see who the code owner is before you open a pull request. In a Gitea repository, you can browse to the file and hover over the shield icon to see a tool tip with code ownership details. You can also see this code ownership shield for individual files when opening a pull request, viewing a pull request's changed files, or viewing a commit.

## CODEOWNRS File Location
To use a CODEOWNERS file, create a new file called `CODEOWNERS` in the root, `docs/`, or `.gitea/` directory of the repository in the branch where you'd like to add the code owners. There should only be one such file. The first matching file is used to determine ownership and the rest are ignored.

Each CODEOWNERS file assigns the code owners for a single branch in the repository. Thus, you can assign different code owners for different branches.

For code owners to receive review requests, the CODEOWNERS file must be on the base branch of the pull request. For example, if you assign `@userA` as the code owner for *.js* files on the `feature-A` branch of your repository, `@userA` will receive review requests when a pull request with changes to *.js* files is opened between the head branch and `feature-A`.

## CODEOWNERS File Size
CODEOWNERS files must be under 3 MB in size. A CODEOWNERS file over this limit will not be loaded, which means that code owner information is not shown and the appropriate code owners will not be requested to review changes in a pull request.

To reduce the size of your CODEOWNERS file, consider using wildcard patterns to consolidate multiple entries into a single entry.

## CODEOWNERS Syntax
> **Warning**: There are some syntax rules for gitignore files that *do not work* in CODEOWNERS files:
> * Escaping a pattern starting with `#` using `\` so it is treated as a pattern and not a comment
> * Using `!` to negate a pattern
> * Using `[ ]` to define a character range

A CODEOWNERS file uses a pattern that follows most of the same rules used in [gitignore](https://git-scm.com/docs/gitignore#_pattern_format) files. The pattern is followed by one or more Gitea usernames or team names using the standard `@username` or `@org/team-name` format. Users must have explicit `write` access to the repository. Teams must also have explicit `write` access to the repository, even if the all the team's members already have access.

If you want to match two or more code owners with the same pattern, all the code owners must be on the same line. If the code owners are not on the same line, the pattern matches only the last mentioned code owner.

You can also refer to a user by their email address (for example, `user@example.com`). Note that if their email address changes after the CODEOWNERS file is created, they would fail to be identified when parsing the file.

CODEOWNERS paths are case sensitive, because Gitea uses a case sensitive file system. Since CODEOWNERS are evaluated by Gitea, even systems that are case insensitive (for example, macOS) must use paths and files that are cased correctly in the CODEOWNERS file.

If any line in your CODEOWNERS file contains invalid syntax or references a user or team that is ineligible, that line will be skipped. When you navigate to the CODEOWNERS file in your repository on Gitea, you can see any validation errors.

### Example CODEOWNERS file
```
# This is a comment.
# Each line is a file pattern followed by one or more owners.

# These owners will be the default owners for everything in the repo. Unless a later match takes precedence,
# @global-owner1 and @global-owner2 will be requested for review when someone opens a pull request.
* @global-owner1 @global-owner2

# Order is important; the last matching pattern takes the most precedence. When someone opens a pull request
# that only modifies JS files, only @js-owner and not the global owner(s) will be requested for a review.
*.js @js-owner #This is an inline comment.

# You can also use email addresses if you prefer.
*.go docs@example.com

# Teams can be specified as code owners as well. Teams should be identified in the format @org/team-name. Teams must have
# explicit write access to the repository. In this example, the octocats team in the octo-org organization owns all .txt files.
*.txt @octo-org/octocats

# In this example, @doctocat owns any files in the build/logs
# directory at the root of the repository and any of its subdirectories.
/build/logs/ @doctocat

# The `docs/*` pattern will match files like `docs/getting-started.md` but not further
# nested files like `docs/build-app/troubleshooting.md`.
docs/*  docs@example.com

# In this example, @octocat owns any file in an apps directory anywhere in your repository.
apps/ @octocat

# In this example, @doctocat owns any file in the `/docs` directory in the root of your repository and any of its subdirectories.
/docs/ @doctocat

# In this example, any change inside the `/scripts` directory will require approval from @doctocat or @octocat.
/scripts/ @doctocat @octocat

# In this example, @octocat owns any file in a `/logs` directory such as `/build/logs`, `/scripts/logs`, and
# `/deeply/nested/logs`. Any changes in a `/logs` directory will require approval from @octocat.
**/logs @octocat

# In this example, @octocat owns any file in the `/apps` directory in the root of your repository
# except for the `/apps/github` subdirectory, as its owners are left empty.
/apps/ @octocat
/apps/github
```