---
date: "2019-04-19:44:00+01:00"
title: "OAuth2 提供者"
slug: "oauth2-provider"
weight: 41
toc: false
draft: false
menu:
  sidebar:
    parent: "development"
    name: "OAuth2 提供者"
    weight: 41
    identifier: "oauth2-provider"
---

# OAuth2 提供者

**目录**

{{< toc >}}

Gitea 支持作为 OAuth2 提供者，能够让第三方应用在用户同意的情况下读写 Gitea 的资源。此功能自 1.8.0 版本开始支持。

## Endpoint

| Endpoint               | URL                         |
| ---------------------- | --------------------------- |
| Authorization Endpoint | `/login/oauth/authorize`    |
| Access Token Endpoint  | `/login/oauth/access_token` |

## 支持的 OAuth2 Grant

目前 Gitea 只支持 [**Authorization Code Grant**](https://tools.ietf.org/html/rfc6749#section-1.3.1) 标准，并额外支持下列扩充标准：

- [Proof Key for Code Exchange (PKCE)](https://tools.ietf.org/html/rfc7636)
- [OpenID Connect (OIDC)](https://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth)

若想要让第三方应用使用 Authorization Code Grant，需要先在「设置」(`/user/settings/applications`)中注册一个新的应用程序。

## Scope

目前 Gitea 尚未支持 scope （参见 [#4300](https://github.com/go-gitea/gitea/issues/4300)），所有的第三方应用都可获得该用户以及他所属的组织中所有资源的读写权。

## 示例

**备注：** 此示例未使用 PKCE。

1. 重定向用户到 authorization endpoint 以获得他同意授权读写资源：
    <!-- 1. Redirect to user to the authorization endpoint in order to get their consent for accessing the resources: -->

   ```curl
   https://[YOUR-GITEA-URL]/login/oauth/authorize?client_id=CLIENT_ID&redirect_uri=REDIRECT_URI& response_type=code&state=STATE
   ```

   在设置中注册应用程序以获得 `CLIENT_ID`。`STATE` 是一个随机字符串，它将在获得用户授权后发送回您的应用程序。`state` 参数是可选的，但您应该使用它来防止 CSRF 攻击。

   ![Authorization Page](/authorize.png)

   用户将会被询问是否授权给您的应用程序。如果他们同意了授权，用户将会被重定向到 `REDIRECT_URL`，例如：

   ```curl
   https://[REDIRECT_URI]?code=RETURNED_CODE&state=STATE
   ```

1. 使用重定向提供的 `code`，您可以请求一个新的应用程序和 Refresh Token。Access Token Endpoint 接受 `application/json` 或 `application/x-www-form-urlencoded` 类型的 POST 请求，例如：

   ```curl
   POST https://[YOUR-GITEA-URL]/login/oauth/access_token
   ```

   ```json
   {
     "client_id": "YOUR_CLIENT_ID",
     "client_secret": "YOUR_CLIENT_SECRET",
     "code": "RETURNED_CODE",
     "grant_type": "authorization_code",
     "redirect_uri": "REDIRECT_URI"
   }
   ```

   返回：

   ```json
   {
     "access_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjowLCJleHAiOjE1NTUxNzk5MTIsImlhdCI6MTU1NTE3NjMxMn0.0-iFsAwBtxuckA0sNZ6QpBQmywVPz129u75vOM7wPJecw5wqGyBkmstfJHAjEOqrAf_V5Z-1QYeCh_Cz4RiKug",
     "token_type": "bearer",
     "expires_in": 3600,
     "refresh_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjoxLCJjbnQiOjEsImV4cCI6MTU1NzgwNDMxMiwiaWF0IjoxNTU1MTc2MzEyfQ.S_HZQBy4q9r5SEzNGNIoFClT43HPNDbUdHH-GYNYYdkRfft6XptJBkUQscZsGxOW975Yk6RbgtGvq1nkEcklOw"
   }
   ```

   `CLIENT_SECRET` 是生成给应用程序的唯一密钥。请记住，该密钥只会在您使用 Gitea 创建/注册应用程序后出现一次。如果您丢失了密钥，您必须在应用程序设置中重新生成密钥。

   `access_token` 请求中的 `REDIRECT_URI` 必须符合 `authorize` 请求中的 `REDIRECT_URI`。

1.发送 [API requests](https://docs.gitea.io/en-us/api-usage#oauth2) 时使用 `access_token` 以读写用户的资源。
