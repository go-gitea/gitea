---
date: "2016-12-01T16:00:00+02:00"
title: "认证"
slug: "authentication"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "认证"
    weight: 10
    identifier: "authentication"
---

# 认证

## 反向代理认证

Gitea 支持通过读取反向代理传递的 HTTP 头中的登录名或者 email 地址来支持反向代理来认证。默认是不启用的，你可以用以下配置启用。

```ini
[service]
ENABLE_REVERSE_PROXY_AUTHENTICATION = true
```

默认的登录用户名的 HTTP 头是 `X-WEBAUTH-USER`，你可以通过修改 `REVERSE_PROXY_AUTHENTICATION_USER` 来变更它。如果用户不存在，可以自动创建用户，当然你需要修改 `ENABLE_REVERSE_PROXY_AUTO_REGISTRATION=true` 来启用它。

默认的登录用户 Email 的 HTTP 头是 `X-WEBAUTH-EMAIL`，你可以通过修改 `REVERSE_PROXY_AUTHENTICATION_EMAIL` 来变更它。如果用户不存在，可以自动创建用户，当然你需要修改 `ENABLE_REVERSE_PROXY_AUTO_REGISTRATION=true` 来启用它。你也可以通过修改 `ENABLE_REVERSE_PROXY_EMAIL` 来启用或停用这个 HTTP 头。

如果设置了 `ENABLE_REVERSE_PROXY_FULL_NAME=true`，则用户的全名会从 `X-WEBAUTH-FULLNAME` 读取，这样在自动创建用户时将使用这个字段作为用户全名，你也可以通过修改 `REVERSE_PROXY_AUTHENTICATION_FULL_NAME` 来变更 HTTP 头。

你也可以通过修改 `REVERSE_PROXY_TRUSTED_PROXIES` 来设置反向代理的IP地址范围，加强安全性，默认值是 `127.0.0.0/8,::1/128`。 通过 `REVERSE_PROXY_LIMIT`， 可以设置最多信任几级反向代理。

注意：反向代理认证不支持认证 API，API 仍旧需要用 access token 来进行认证。
