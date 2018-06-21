---
date: "2018-06-19T16:00:00+02:00"
title: "API Usage"
slug: "api-usage"
weight: 40
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "API Usage"
    weight: 40
    identifier: "api-usage"
---

## API Usage

### /users/{username}/tokens

Usage:
```
$ curl --request GET --url https://m:yourpassword@git.your.host/api/v1/users/m/tokens
```

Response:
```
[{"name":"test","sha1":"..."},{"name":"dev","sha1":"..."}]
```
