---
date: "2017-04-08T11:34:00+02:00"
title: "Environment variables"
slug: "environment-variables"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "administration"
    name: "Environment variables"
    weight: 10
    identifier: "environment-variables"
---

# Environment variables

**Table of Contents**

{{< toc >}}

This is an inventory of Gitea environment variables. They change Gitea behaviour.

Initialize them before Gitea command to be effective, for example:

```sh
GITEA_CUSTOM=/home/gitea/custom ./gitea web
```

## From Go language

As Gitea is written in Go, it uses some Go variables, such as:

- `GOOS`
- `GOARCH`
- [`GOPATH`](https://golang.org/cmd/go/#hdr-GOPATH_environment_variable)

For documentation about each of the variables available, refer to the
[official Go documentation](https://golang.org/cmd/go/#hdr-Environment_variables).

## Gitea files

- `GITEA_WORK_DIR`: Absolute path of working directory.
- `GITEA_CUSTOM`: Gitea uses `GITEA_WORK_DIR`/custom folder by default. Use this variable
  to change _custom_ directory.
- `GOGS_WORK_DIR`: Deprecated, use `GITEA_WORK_DIR`
- `GOGS_CUSTOM`: Deprecated, use `GITEA_CUSTOM`

## Operating system specifics

- `USER`: System user that Gitea will run as. Used for some repository access strings.
- `USERNAME`: if no `USER` found, Gitea will use `USERNAME`
- `HOME`: User home directory path. The `USERPROFILE` environment variable is used in Windows.

### Only on Windows

- `USERPROFILE`: User home directory path. If empty, uses `HOMEDRIVE` + `HOMEPATH`
- `HOMEDRIVE`: Main drive path used to access the home directory (C:)
- `HOMEPATH`: Home relative path in the given home drive path

## Miscellaneous

- `SKIP_MINWINSVC`: If set to 1, do not run as a service on Windows.
