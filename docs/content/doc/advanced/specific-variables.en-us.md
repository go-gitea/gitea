---
date: "2017-04-08T11:34:00+02:00"
title: "Specific variables"
slug: "specific-variables"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Specific variables"
    weight: 20
    identifier: "specific-variables"
---

# Specific variables

This is an inventory of Gitea environment variables. They change Gitea behaviour.

Initialize them before Gitea command to be effective, for example:

```
GITEA_CUSTOM=/home/gitea/custom ./gitea web
```

## From Go language

As Gitea is written in Go, it uses some Go variables as:

  * `GOOS`
  * `GOARCH`
  * `GOPATH`

For `GOPATH`, check [official documentation about GOPATH environment variable](https://golang.org/cmd/go/#hdr-GOPATH_environment_variable).

For others, check [official documentation about variables used when it runs the generator](https://golang.org/cmd/go/#hdr-Generate_Go_files_by_processing_source).

## Gitea files

  * `GITEA_WORK_DIR`: Gitea absolute path of work directory.
  * `GITEA_CUSTOM`: Gitea uses `GITEA_WORK_DIR`/custom folder by default. Use this variable to change *custom* directory.
  * `GOGS_WORK_DIR`: Deprecated, use `GITEA_WORK_DIR`
  * `GOGS_CUSTOM`: Deprecated, use `GITEA_CUSTOM`

## Operating system specifics

  * `USER`: system user that launch Gitea. Useful for repository URL address on Gitea interface
  * `USERNAME`: if no USER found, Gitea will try `USERNAME`
  * `HOME`: User home directory path (**except if** you're running on Windows, check  the following `USERPROFILE` variable)

### Only on Windows

  * `USERPROFILE`: User home directory path. If empty, uses `HOMEDRIVE` + `HOMEPATH`
  * `HOMEDRIVE`: Main drive path you will use to get home directory
  * `HOMEPATH`: Home relative path in the given home drive path

## Macaron (framework used by Gitea)

  * `HOST`: Host Macaron will listen on
  * `PORT`: Port Macaron will listen on
  * `MACARON_ENV`: global variable to provide special functionality for development environments vs production environments. If MACARON_ENV is set to "" or "development" then templates will be recompiled on every request. For more performance, set the MACARON_ENV environment variable to "production".

## Miscellaneous

  * `SKIP_MINWINSVC`: Do not run as a service on Windows if set to 1
  * `ZOOKEEPER_PATH`: [Zookeeper](http://zookeeper.apache.org/) jar file path

