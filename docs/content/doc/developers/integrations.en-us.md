---
date: "2019-04-15T17:29:00+08:00"
title: "Web UI API"
slug: "integrations"
weight: 40
toc: false
draft: false
menu:
  sidebar:
    parent: "developers"
    name: "Web UI API"
    weight: 65
    identifier: "integrations"
---

# Web UI API

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
