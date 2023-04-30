---
date: "2019-04-19:44:00+01:00"
title: "OAuth2 提供者"
slug: "oauth2-provider"
weight: 41
toc: false
draft: false
aliases:
  - /zh-cn/oauth2-provider
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

Gitea 支持作为 OAuth2 提供者，允许第三方应用程序在用户同意的情况下访问其资源。此功能自 1.8.0 版起可用。

## 端点

| 端点                     | URL                                 |
| ------------------------ | ----------------------------------- |
| OpenID Connect Discovery | `/.well-known/openid-configuration` |
| Authorization Endpoint   | `/login/oauth/authorize`            |
| Access Token Endpoint    | `/login/oauth/access_token`         |
| OpenID Connect UserInfo  | `/login/oauth/userinfo`             |
| JSON Web Key Set         | `/login/oauth/keys`                 |

## 支持的 OAuth2 授权

目前 Gitea 仅支持 [**Authorization Code Grant**](https://tools.ietf.org/html/rfc6749#section-1.3.1) 标准，并额外支持以下扩展：

- [Proof Key for Code Exchange (PKCE)](https://tools.ietf.org/html/rfc7636)
- [OpenID Connect (OIDC)](https://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth)

要将 Authorization Code Grant 作为第三方应用程序，您需要通过在设置中添加一个新的 "应用程序" (`/user/settings/applications`)。

## 范围

Gitea 支持以下令牌范围:

| 名称 | 介绍 |
| ---- | ----------- |
| **(no scope)** | 授予对公共用户配置文件和公共存储库的只读访问权限 |
| **repo** | 完全控制所有存储库 |
| &nbsp;&nbsp;&nbsp; **repo:status** | 授予对所有存储库中提交状态的读/写访问权限 |
| &nbsp;&nbsp;&nbsp; **public_repo** | 仅授予对公共存储库的读/写访问权限 |
| **admin:repo_hook** | 授予对所有存储库的 Hooks 访问权限，该权限已包含在 `repo` 范围中 |
| &nbsp;&nbsp;&nbsp; **write:repo_hook** | 授予对存储库 Hooks 的读/写访问权限 |
| &nbsp;&nbsp;&nbsp; **read:repo_hook** | 授予对存储库 Hooks 的只读访问权限 |
| **admin:org** | 授予对组织设置的完全访问权限 |
| &nbsp;&nbsp;&nbsp; **write:org** | 授予对组织设置的读/写访问权限 |
| &nbsp;&nbsp;&nbsp; **read:org** | 授予对组织设置的只读访问权限 |
| **admin:public_key** | 授予公钥管理的完全访问权限 |
| &nbsp;&nbsp;&nbsp; **write:public_key** | 授予对公钥的读/写访问权限 |
| &nbsp;&nbsp;&nbsp; **read:public_key** | 授予对公钥的只读访问权限 |
| **admin:org_hook** | 授予对组织级别 Hooks 的完全访问权限 |
| **admin:user_hook** | 授予对用户级别 Hooks 的完全访问权限 |
| **notification** | 授予对通知的完全访问权限 |
| **user** | 授予对用户个人资料信息的完全访问权限 |
| &nbsp;&nbsp;&nbsp; **read:user** | 授予对用户个人资料的读取权限 |
| &nbsp;&nbsp;&nbsp; **user:email** | 授予对用户电子邮件地址的读取权限 |
| &nbsp;&nbsp;&nbsp; **user:follow** | 授予访问权限以关注/取消关注用户 |
| **delete_repo** | 授予删除存储库的权限 |
| **package** | 授予对托管包的完全访问权限 |
| &nbsp;&nbsp;&nbsp; **write:package** | 授予对包的读/写访问权限 |
| &nbsp;&nbsp;&nbsp; **read:package** | 授予对包的读取权限 |
| &nbsp;&nbsp;&nbsp; **delete:package** | 授予对包的删除权限 |
| **admin:gpg_key** | 授予 GPG 密钥管理的完全访问权限 |
| &nbsp;&nbsp;&nbsp; **write:gpg_key** | 授予对 GPG 密钥的读/写访问权限 |
| &nbsp;&nbsp;&nbsp; **read:gpg_key** | 授予对 GPG 密钥的只读访问权限 |
| **admin:application** | 授予应用程序管理的完全访问权限 |
| &nbsp;&nbsp;&nbsp; **write:application** | 授予应用程序管理的读/写访问权限 |
| &nbsp;&nbsp;&nbsp; **read:application** | 授予应用程序管理的读取权限 |
| **sudo** | 允许以站点管理员身份执行操作 |

## 客户端类型

Gitea 支持私密和公共客户端类型，[参见 RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749#section-2.1).

对于公共客户端, 允许在本地回环地址的重定向 URI 中使用任意端口，例如 `http://127.0.0.1/`。根据 [RFC 8252 的建议](https://datatracker.ietf.org/doc/html/rfc8252#section-8.3)，请避免使用 `localhost`。

## 示例

**注意：** 该示例中尚未使用 PKCE。

1. 将用户重定向到授权端点，以获得他们的访问资源授权:

   ```curl
   https://[YOUR-GITEA-URL]/login/oauth/authorize?client_id=CLIENT_ID&redirect_uri=REDIRECT_URI& response_type=code&state=STATE
   ```

   在设置中注册应用程序以获得 `CLIENT_ID`。`STATE` 是一个随机字符串，它将在获得用户授权后发送回您的应用程序。`state` 参数是可选的，但您应该使用它来防止 CSRF 攻击。

   ![Authorization Page](/authorize.png)

   用户将会被询问是否授权给您的应用程序。如果他们同意了授权，用户将会被重定向到 `REDIRECT_URL`，例如：

   ```curl
   https://[REDIRECT_URI]?code=RETURNED_CODE&state=STATE
   ```

2. 使用重定向提供的 `code`，您可以请求一个新的应用程序和 Refresh Token。Access Token Endpoint 接受 `application/json` 或 `application/x-www-form-urlencoded` 类型的 POST 请求，例如：

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

   `CLIENT_SECRET` 是生成给应用程序的唯一密钥。请注意，该密钥只会在您使用 Gitea 创建/注册应用程序后出现一次。如果您丢失了密钥，您必须在应用程序设置中重新生成密钥。

   `access_token` 请求中的 `REDIRECT_URI` 必须与 `authorize` 请求中的 `REDIRECT_URI` 相符。

3. 使用 `access_token` 来构造 [API 请求](https://docs.gitea.io/en-us/api-usage#oauth2) 以读写用户的资源。
