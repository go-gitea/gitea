---
date: "2023-05-24T16:00:00+00:00"
title: "Code Owners"
slug: "code-owners"
weight: 30
toc: false
draft: false
aliases:
  - /en-us/code-owners
menu:
  sidebar:
    parent: "usage"
    name: "Code Owners"
    weight: 30
    identifier: "code-owners"
---

# Code Owners

Gitea maintains code owner files. It looks for it in the following locations in this order:

- `./CODEOWNERS`
- `./docs/CODEOWNERS`
- `./.gitea/CODEOWNERS`

And stops at the first found file.

File format: `<regexp rule> <@user or @org/team> [@user or @org/team]...`

Regexp specified in golang Regex format.
Regexp can start with `!` for negative rules - match all files except specified.

Example file:

```
.*\\.go @user1 @user2 # This is comment

# Comment too
# You can assigning code owning for users or teams
frontend/src/.*\\.js @org1/team1 @org1/team2 @user3

# You can use negative pattern
!frontend/src/.* @org1/team3 @user5

# You can use power of go regexp
docs/(aws|google|azure)/[^/]*\\.(md|txt) @user8 @org1/team4
!/assets/.*\\.(bin|exe|msi) @user9
```

### Escaping

You can escape characters `#`, ` ` (space) and `\` with `\`, like:

```
dir/with\#hashtag @user1
path\ with\ space @user2
path/with\\backslash @user3
```

Some character (`.+*?()|[]{}^$\`) should be escaped with `\\` inside regexp, like:

```
path/\\.with\\.dots
path/with\\+plus
```
