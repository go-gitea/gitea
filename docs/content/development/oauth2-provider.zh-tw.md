---
date: "2019-04-19:44:00+01:00"
title: "OAuth2 提供者"
slug: "oauth2-provider"
sidebar_position: 41
toc: false
draft: false
aliases:
  - /zh-tw/oauth2-provider
menu:
  sidebar:
    parent: "development"
    name: "OAuth2 提供者"
    sidebar_position: 41
    identifier: "oauth2-provider"
---

# OAuth2 提供者

**目錄**

Gitea 支援作為 OAuth2 提供者，能讓第三方程式能在使用者同意下存取 Gitea 的資源。此功能自 1.8.0 版開始提供。

## Endpoint

| Endpoint               | URL                         |
| ---------------------- | --------------------------- |
| Authorization Endpoint | `/login/oauth/authorize`    |
| Access Token Endpoint  | `/login/oauth/access_token` |

## 支援的 OAuth2 Grant

目前 Gitea 只支援 [**Authorization Code Grant**](https://tools.ietf.org/html/rfc6749#section-1.3.1) 標準並額外支援下列擴充標準：

- [Proof Key for Code Exchange (PKCE)](https://tools.ietf.org/html/rfc7636)
- [OpenID Connect (OIDC)](https://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth)

若想要讓第三方程式使用 Authorization Code Grant，需先在「設定」(`/user/settings/applications`)中註冊一個新的應用程式。

## Scope

目前 Gitea 尚未支援 scope （參見 [#4300](https://github.com/go-gitea/gitea/issues/4300)），所有的第三方程式都可獲得該使用者及他所屬的組織中所有資源的存取權。

## 範例

**備註：** 此範例未使用 PKCE。

1. 重新導向使用者到 authorization endpoint 以獲得他同意授權存取資源：
    <!-- 1. Redirect to user to the authorization endpoint in order to get their consent for accessing the resources: -->

   ```curl
   https://[YOUR-GITEA-URL]/login/oauth/authorize?client_id=CLIENT_ID&redirect_uri=REDIRECT_URI& response_type=code&state=STATE
   ```

   在設定中註冊應用程式以獲得 `CLIENT_ID`。`STATE` 是一個隨機的字串，它將在使用者授權後發送回您的應用程式。`state` 參數是選用的，但應該要用它來防止 CSRF 攻擊。

   ![Authorization Page](/authorize.png)

   使用者將會被詢問是否授權給您的應用程式。如果它們同意了，使用者將被重新導向到 `REDIRECT_URL`，例如：

   ```curl
   https://[REDIRECT_URI]?code=RETURNED_CODE&state=STATE
   ```

1. 使用重新導向提供的 `code`，您可以要求一個新的應用程式和 Refresh Token。Access Token Endpoint 接受 POST 請求使用 `application/json` 或 `application/x-www-form-urlencoded` 類型的請求內容，例如：

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

   回應：

   ```json
   {
     "access_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjowLCJleHAiOjE1NTUxNzk5MTIsImlhdCI6MTU1NTE3NjMxMn0.0-iFsAwBtxuckA0sNZ6QpBQmywVPz129u75vOM7wPJecw5wqGyBkmstfJHAjEOqrAf_V5Z-1QYeCh_Cz4RiKug",
     "token_type": "bearer",
     "expires_in": 3600,
     "refresh_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjoxLCJjbnQiOjEsImV4cCI6MTU1NzgwNDMxMiwiaWF0IjoxNTU1MTc2MzEyfQ.S_HZQBy4q9r5SEzNGNIoFClT43HPNDbUdHH-GYNYYdkRfft6XptJBkUQscZsGxOW975Yk6RbgtGvq1nkEcklOw"
   }
   ```

   `CLIENT_SECRET` 是產生給此應用程式的唯一密鑰。請記住該密鑰只會在您於 Gitea 建立/註冊應用程式時出現一次。若您遺失密鑰，您必須在該應用程式的設定中重新產生密鑰。

   `access_token` 請求中的 `REDIRECT_URI` 必須符合 `authorize` 請求中的 `REDIRECT_URI`。

1. 發送 [API requests](development/api-usage.md#oauth2-provider) 時使用 `access_token` 以存取使用者的資源。
