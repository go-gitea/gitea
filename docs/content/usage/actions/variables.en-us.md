---
date: "2024-04-10T22:21:00+08:00"
title: "Variables"
slug: "actions-variables"
sidebar_position: 25
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "Variables"
    sidebar_position: 25
    identifier: "actions-variables"
---

## Variables

You can create configuration variables on the user, organization and repository level.
The level of the variable depends on where you created it. When creating a variable, the
key will be converted to uppercase. You need use uppercase on the yaml file.

### Naming conventions

The following rules apply to variable names:

- Variable names can only contain alphanumeric characters (`[a-z]`, `[A-Z]`, `[0-9]`) or underscores (`_`). Spaces are not allowed.
- Variable names must not start with the `GITHUB_` and `GITEA_` prefix.
- Variable names must not start with a number.
- Variable names are case-insensitive.
- Variable names must be unique at the level they are created at.
- Variable names must not be `CI`.

### Using variable

After creating configuration variables, they will be automatically filled in the `vars` context.
They can be accessed through expressions like `${{ vars.VARIABLE_NAME }}` in the workflow.

### Precedence

If a variable with the same name exists at multiple levels, the variable at the lowest level takes precedence:
A repository variable will always be chosen over an organization/user variable.
