---
date: "2023-06-01T08:40:00+08:00"
title: "OAuth2 provider"
slug: "oauth2-provider"
sidebar_position: 41
toc: false
draft: false
aliases:
  - /en-us/oauth2-provider
menu:
  sidebar:
    parent: "development"
    name: "OAuth2 Provider"
    sidebar_position: 41
    identifier: "oauth2-provider"
---

# OAuth2 provider

Gitea supports acting as an OAuth2 provider to allow third party applications to access its resources with the user's consent. This feature is available since release 1.8.0.

## Endpoints

| Endpoint                 | URL                                 |
| ------------------------ | ----------------------------------- |
| OpenID Connect Discovery | `/.well-known/openid-configuration` |
| Authorization Endpoint   | `/login/oauth/authorize`            |
| Access Token Endpoint    | `/login/oauth/access_token`         |
| OpenID Connect UserInfo  | `/login/oauth/userinfo`             |
| JSON Web Key Set         | `/login/oauth/keys`                 |

## Supported OAuth2 Grants

At the moment Gitea only supports the [**Authorization Code Grant**](https://tools.ietf.org/html/rfc6749#section-1.3.1) standard with additional support of the following extensions:

- [Proof Key for Code Exchange (PKCE)](https://tools.ietf.org/html/rfc7636)
- [OpenID Connect (OIDC)](https://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth)

To use the Authorization Code Grant as a third party application it is required to register a new application via the "Settings" (`/user/settings/applications`) section of the settings. To test or debug you can use the web-tool https://oauthdebugger.com/.

## Scopes

Gitea supports scoped access tokens, which allow users the ability to restrict tokens to operate only on selected url routes. Scopes are grouped by high-level API routes, and further refined to the following:

- `read`: `GET` routes
- `write`: `POST`, `PUT`, `PATCH`, and `DELETE` routes (in addition to `GET`)

Gitea token scopes are as follows:

| Name | Description                                                                                                                                          |
| ---- |------------------------------------------------------------------------------------------------------------------------------------------------------|
| **(no scope)** | Not supported. A scope is required even for public repositories.                                                                                     |
| **activitypub** | `activitypub` API routes: ActivityPub related operations.                                                                                            |
| &nbsp;&nbsp;&nbsp; **read:activitypub** | Grants read access for ActivityPub operations.                                                                                                       |
| &nbsp;&nbsp;&nbsp; **write:activitypub** | Grants read/write/delete access for ActivityPub operations.                                                                                          |
| **admin** | `/admin/*` API routes: Site-wide administrative operations (hidden for non-admin accounts).                                                          |
| &nbsp;&nbsp;&nbsp; **read:admin** | Grants read access for admin operations, such as getting cron jobs or registered user emails.                                                        |
| &nbsp;&nbsp;&nbsp; **write:admin** | Grants read/write/delete access for admin operations, such as running cron jobs or updating user accounts.                                           |
| **issue** | `issues/*`, `labels/*`, `milestones/*` API routes: Issue-related operations.                                                                         |
| &nbsp;&nbsp;&nbsp; **read:issue** | Grants read access for issues operations, such as getting issue comments, issue attachments, and milestones.                                         |
| &nbsp;&nbsp;&nbsp; **write:issue** | Grants read/write/delete access for issues operations, such as posting or editing an issue comment or attachment, and updating milestones.           |
| **misc** | Reserved for future usage.                                                                                                                           |
| &nbsp;&nbsp;&nbsp; **read:misc** | Reserved for future usage.                                                                                                                           |
| &nbsp;&nbsp;&nbsp; **write:misc** | Reserved for future usage.                                                                                                                           |
| **notification** | `notification/*` API routes: user notification operations.                                                                                           |
| &nbsp;&nbsp;&nbsp; **read:notification** | Grants read access to user notifications, such as which notifications users are subscribed to and read new notifications.                            |
| &nbsp;&nbsp;&nbsp; **write:notification** | Grants read/write/delete access to user notifications, such as marking notifications as read.                                                        |
| **organization** | `orgs/*` and `teams/*` API routes: Organization and team management operations.                                                                      |
| &nbsp;&nbsp;&nbsp; **read:organization** | Grants read access to org and team status, such as listing all orgs a user has visibility to, teams, and team members.                               |
| &nbsp;&nbsp;&nbsp; **write:organization** | Grants read/write/delete access to org and team status, such as creating and updating teams and updating org settings.                               |
| **package** | `/packages/*` API routes: Packages operations                                                                                                        |
| &nbsp;&nbsp;&nbsp; **read:package** | Grants read access to package operations, such as reading and downloading available packages.                                                        |
| &nbsp;&nbsp;&nbsp; **write:package** | Grants read/write/delete access to package operations. Currently the same as `read:package`.                                                         |
| **repository** | `/repos/*` API routes except `/repos/issues/*`: Repository file, pull-request, and release operations.                                               |
| &nbsp;&nbsp;&nbsp; **read:repository** | Grants read access to repository operations, such as getting repository files, releases, collaborators.                                              |
| &nbsp;&nbsp;&nbsp; **write:repository** | Grants read/write/delete access to repository operations, such as getting updating repository files, creating pull requests, updating collaborators. |
| **user** | `/user/*` and `/users/*` API routes: User-related operations.                                                                                        |
| &nbsp;&nbsp;&nbsp; **read:user** | Grants read access to user operations, such as getting user repo subscriptions and user settings.                                                    |
| &nbsp;&nbsp;&nbsp; **write:user** | Grants read/write/delete access to user operations, such as updating user repo subscriptions, followed users, and user settings.                     |

## Pre-configured Applications

Gitea creates OAuth applications for the following services by default on startup, as we assume that these are universally useful.

|Application|Description|Client ID|
|-----------|-----------|---------|
|[git-credential-oauth](https://github.com/hickford/git-credential-oauth)|Git credential helper|`a4792ccc-144e-407e-86c9-5e7d8d9c3269`|
|[Git Credential Manager](https://github.com/git-ecosystem/git-credential-manager)|Git credential helper|`e90ee53c-94e2-48ac-9358-a874fb9e0662`|
|[tea](https://gitea.com/gitea/tea)|tea|`d57cb8c4-630c-4168-8324-ec79935e18d4`|

To prevent unexpected behavior, they are being displayed as locked in the UI and their creation can instead be controlled by the `DEFAULT_APPLICATIONS` parameter in `app.ini`.

## Client types

Gitea supports both confidential and public client types, [as defined by RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749#section-2.1).

For public clients, a redirect URI of a loopback IP address such as `http://127.0.0.1/` allows any port. Avoid using `localhost`, [as recommended by RFC 8252](https://datatracker.ietf.org/doc/html/rfc8252#section-8.3).

## Examples

### Confidential client

**Note:** This example does not use PKCE.

1. Redirect the user to the authorization endpoint in order to get their consent for accessing the resources:

   ```curl
   https://[YOUR-GITEA-URL]/login/oauth/authorize?client_id=CLIENT_ID&redirect_uri=REDIRECT_URI&response_type=code&state=STATE
   ```

   The `CLIENT_ID` can be obtained by registering an application in the settings. The `STATE` is a random string that will be sent back to your application after the user authorizes. The `state` parameter is optional, but should be used to prevent CSRF attacks.

   ![Authorization Page](/authorize.png)

   The user will now be asked to authorize your application. If they authorize it, the user will be redirected to the `REDIRECT_URL`, for example:

   ```curl
   https://[REDIRECT_URI]?code=RETURNED_CODE&state=STATE
   ```

2. Using the provided `code` from the redirect, you can request a new application and refresh token. The access token endpoint accepts POST requests with `application/json` and `application/x-www-form-urlencoded` body, for example:

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

   Response:

   ```json
   {
     "access_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjowLCJleHAiOjE1NTUxNzk5MTIsImlhdCI6MTU1NTE3NjMxMn0.0-iFsAwBtxuckA0sNZ6QpBQmywVPz129u75vOM7wPJecw5wqGyBkmstfJHAjEOqrAf_V5Z-1QYeCh_Cz4RiKug",
     "token_type": "bearer",
     "expires_in": 3600,
     "refresh_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjoxLCJjbnQiOjEsImV4cCI6MTU1NzgwNDMxMiwiaWF0IjoxNTU1MTc2MzEyfQ.S_HZQBy4q9r5SEzNGNIoFClT43HPNDbUdHH-GYNYYdkRfft6XptJBkUQscZsGxOW975Yk6RbgtGvq1nkEcklOw"
   }
   ```

   The `CLIENT_SECRET` is the unique secret code generated for this application. Please note that the secret will only be visible after you created/registered the application with Gitea and cannot be recovered. If you lose the secret, you must regenerate the secret via the application's settings.

   The `REDIRECT_URI` in the `access_token` request must match the `REDIRECT_URI` in the `authorize` request.

3. Use the `access_token` to make [API requests](development/api-usage.md#oauth2-provider) to access the user's resources.

### Public client (PKCE)

PKCE (Proof Key for Code Exchange) is an extension to the OAuth flow which allows for a secure credential exchange without the requirement to provide a client secret.

**Note**: Please ensure you have registered your OAuth application as a public client.

To achieve this, you have to provide a `code_verifier` for every authorization request. A `code_verifier` has to be a random string with a minimum length of 43 characters and a maximum length of 128 characters. It can contain alphanumeric characters as well as the characters `-`, `.`, `_`  and `~`.

Using this `code_verifier` string, a new one called `code_challenge` is created by using one of two methods:

- If you have the required functionality on your client, set `code_challenge` to be a URL-safe base64-encoded string of the SHA256 hash of `code_verifier`. In that case, your `code_challenge_method` becomes `S256`.
- If you are unable to do so, you can provide your `code_verifier` as a plain string to `code_challenge`. Then you have to set your `code_challenge_method` as `plain`.

After you have generated this values, you can continue with your request.

1. Redirect the user to the authorization endpoint in order to get their consent for accessing the resources:

   ```curl
   https://[YOUR-GITEA-URL]/login/oauth/authorize?client_id=CLIENT_ID&redirect_uri=REDIRECT_URI&response_type=code&code_challenge_method=CODE_CHALLENGE_METHOD&code_challenge=CODE_CHALLENGE&state=STATE
   ```

   The `CLIENT_ID` can be obtained by registering an application in the settings. The `STATE` is a random string that will be sent back to your application after the user authorizes. The `state` parameter is optional, but should be used to prevent CSRF attacks.

   ![Authorization Page](/authorize.png)

   The user will now be asked to authorize your application. If they authorize it, the user will be redirected to the `REDIRECT_URL`, for example:

   ```curl
   https://[REDIRECT_URI]?code=RETURNED_CODE&state=STATE
   ```

2. Using the provided `code` from the redirect, you can request a new application and refresh token. The access token endpoint accepts POST requests with `application/json` and `application/x-www-form-urlencoded` body, for example:

   ```curl
   POST https://[YOUR-GITEA-URL]/login/oauth/access_token
   ```

   ```json
   {
     "client_id": "YOUR_CLIENT_ID",
     "code": "RETURNED_CODE",
     "grant_type": "authorization_code",
     "redirect_uri": "REDIRECT_URI",
     "code_verifier": "CODE_VERIFIER",
   }
   ```

   Response:

   ```json
   {
     "access_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjowLCJleHAiOjE1NTUxNzk5MTIsImlhdCI6MTU1NTE3NjMxMn0.0-iFsAwBtxuckA0sNZ6QpBQmywVPz129u75vOM7wPJecw5wqGyBkmstfJHAjEOqrAf_V5Z-1QYeCh_Cz4RiKug",
     "token_type": "bearer",
     "expires_in": 3600,
     "refresh_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjoxLCJjbnQiOjEsImV4cCI6MTU1NzgwNDMxMiwiaWF0IjoxNTU1MTc2MzEyfQ.S_HZQBy4q9r5SEzNGNIoFClT43HPNDbUdHH-GYNYYdkRfft6XptJBkUQscZsGxOW975Yk6RbgtGvq1nkEcklOw"
   }
   ```

   The `REDIRECT_URI` in the `access_token` request must match the `REDIRECT_URI` in the `authorize` request.

3. Use the `access_token` to make [API requests](development/api-usage.md#oauth2-provider) to access the user's resources.
