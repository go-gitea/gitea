---
date: "2018-06-24:00:00+02:00"
title: "API 使用指南"
slug: "api-usage"
weight: 40
toc: false
draft: false
menu:
  sidebar:
    parent: "development"
    name: "API 使用指南"
    weight: 40
    identifier: "api-usage"
---

# Gitea API 使用指南

## 开启/配置 API 访问

通常情况下， `ENABLE_SWAGGER` 默认开启并且参数 `MAX_RESPONSE_ITEMS` 默认为 50。您可以从 [Config Cheat
Sheet](https://docs.gitea.io/en-us/config-cheat-sheet/) 中获取更多配置相关信息。

## 通过 API 认证

Gitea 支持以下几种 API 认证方式：

- HTTP basic authentication 方式
- 通过指定 `token=...` URL 查询参数方式
- 通过指定 `access_token=...` URL 查询参数方式
- 通过指定 `Authorization: token ...` HTTP header 方式

以上提及的认证方法接受相同的 apiKey token 类型，您可以在编码时通过查阅代码更好地理解这一点。
Gitea 调用解析查询参数以及头部信息来获取 token 的代码可以在 [modules/auth/auth.go](https://github.com/go-gitea/gitea/blob/6efdcaed86565c91a3dc77631372a9cc45a58e89/modules/auth/auth.go#L47) 中找到。

您可以通过您的 gitea web 界面来创建 apiKey token：
`Settings | Applications | Generate New Token`.

### 关于 `Authorization:` header

由于一些历史原因，Gitea 需要在 header 的 apiKey token 里引入前缀 `token`，类似于如下形式：

```
Authorization: token 65eaa9c8ef52460d22a93307fe0aee76289dc675
```

以 `curl` 命令为例，它会以如下形式携带在请求中：

```
curl "http://localhost:4000/api/v1/repos/test1/test1/issues" \
    -H "accept: application/json" \
    -H "Authorization: token 65eaa9c8ef52460d22a93307fe0aee76289dc675" \
    -H "Content-Type: application/json" -d "{ \"body\": \"testing\", \"title\": \"test 20\"}" -i
```

正如上例所示，您也可以在 GET 请求中使用同一个 token 并以 `token=` 的查询参数形式携带 token 来进行认证。

## 通过 API 列出您发布的令牌

`/users/:name/tokens` 是一个特殊的接口，需要您使用 basic authentication 进行认证，具体原因在 issue 中
[#3842](https://github.com/go-gitea/gitea/issues/3842#issuecomment-397743346) 有所提及，使用方法如下所示：

### 使用 Basic authentication 认证：

```
$ curl --url https://yourusername:yourpassword@gitea.your.host/api/v1/users/yourusername/tokens
[{"name":"test","sha1":"..."},{"name":"dev","sha1":"..."}]
```

## 使用 Sudo 方式请求 API

此 API 允许管理员借用其他用户身份进行 API 请求。只需在请求中指定查询参数 `sudo=` 或是指定 header 中的 `Sudo:` 为需要使用的用户 username 即可。
