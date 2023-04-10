---
date: "2019-04-15T17:29:00+08:00"
title: "Integrations"
slug: "integrations"
weight: 65
toc: false
draft: false
menu:
  sidebar:
    parent: "development"
    name: "Integrations"
    weight: 65
    identifier: "integrations"
---

# Integrations

Gitea has a wonderful community of third-party integrations, as well as first-class support in various other
projects.

We are curating a list over at [awesome-gitea](https://gitea.com/gitea/awesome-gitea) to track these!

If you are looking for [CI/CD](https://gitea.com/gitea/awesome-gitea#user-content-devops),
an [SDK](https://gitea.com/gitea/awesome-gitea#user-content-sdk),
or even some extra [themes](https://gitea.com/gitea/awesome-gitea#user-content-themes),
you can find them listed in the [awesome-gitea](https://gitea.com/gitea/awesome-gitea) repository!

## Pre-Fill New File name and contents

If you'd like to open a new file with a given name and contents,
you can do so with query parameters:

```txt
GET /{{org}}/{{repo}}/_new/{{filepath}}
    ?filename={{filename}}
    &value={{content}}
```

For example:

```txt
GET https://git.example.com/johndoe/bliss/_new/articles/
    ?filename=hello-world.md
    &value=Hello%2C%20World!
```
