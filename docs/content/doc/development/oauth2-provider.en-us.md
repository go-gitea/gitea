---
date: "2019-04-19:44:00+01:00"
title: "OAuth2 provider"
slug: "oauth2-provider"
weight: 41
toc: false
draft: false
aliases:
  - /en-us/oauth2-provider
menu:
  sidebar:
    parent: "development"
    name: "OAuth2 Provider"
    weight: 41
    identifier: "oauth2-provider"
---

# OAuth2 provider

**Table of Contents**

{{< toc >}}

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

To use the Authorization Code Grant as a third party application it is required to register a new application via the "Settings" (`/user/settings/applications`) section of the settings.

## Scopes

Gitea supports scoped access tokens, which allow users the ability to restrict tokens to operate only on selected url routes. Scopes are grouped by high-level API routes, and further refined to the following:

- `read`: `GET` routes
- `write`: `POST`, `PUT`, and `PATCH` routes (in addition to `GET`)
- `delete`: `DELETE` routes (in addition to `POST`, `PUT`, `PATCH` and `GET`)

Gitea token scopes are as follows:

| Name | Description |
| ---- | ----------- |
| **(no scope)** | Not supported. A scope is required even for public repositories. |
| **activitypub** |`activitypub` API routes: ActivityPub related operations. |
| &nbsp;&nbsp;&nbsp; **read:activitypub** | Grants read access for ActivityPub operations. |
| &nbsp;&nbsp;&nbsp; **write:activitypub** | Grants read/write access for ActivityPub operations. |
| &nbsp;&nbsp;&nbsp; **delete:activitypub** | Grants read/write/delete access for ActivityPub operations. Currently the same as `write:activitypub`. |
| **admin** | `/admin/*` API routes: Site-wide administrative operations (hidden for non-admin accounts). |
| &nbsp;&nbsp;&nbsp; **read:admin** | Grants read access for admin operations, such as getting cron jobs or registered user emails. |
| &nbsp;&nbsp;&nbsp; **write:admin** | Grants read/write access for admin operations, such as running cron jobs or updating user accounts. |
| &nbsp;&nbsp;&nbsp; **delete:admin** | Grants read/write/delete access for admin operations, such as deleting user accounts. |
| **issue** | `issues/*`, `labels/*`, `milestones/*` API routes: Issue-related operations. |
| &nbsp;&nbsp;&nbsp; **read:issue** | Grants read access for issues operations, such as getting issue comments, issue attachments, and milestones. |
| &nbsp;&nbsp;&nbsp; **write:issue** | Grants read/write access for issues operations, such as posting or editing an issue comment or attachment, and updating milestones. |
| &nbsp;&nbsp;&nbsp; **delete:issue** | Grants read/write/delete access for issues operations, such as deleting comments, labels or issue attachments. |
| **misc** | miscellaneous and settings top-level API routes. |
| &nbsp;&nbsp;&nbsp; **read:misc** | Grants read access to miscellaneous operations, such as getting label and gitignore templates. |
| &nbsp;&nbsp;&nbsp; **write:misc** | Grants read/write access to miscellaneous operations, such as markup utility operations. |
| &nbsp;&nbsp;&nbsp; **delete:misc** | Grants read/write/delete access to miscellaneous operations. Currently the same as `write:misc`. |
| **notification** | `notification/*` API routes: user notification operations. |
| &nbsp;&nbsp;&nbsp; **read:notification** | Grants read access to user notifications, such as which notifications users are subscribed to and read new notifications. |
| &nbsp;&nbsp;&nbsp; **write:notification** | Grants read/write access to user notifications, such as marking notifications as read. |
| &nbsp;&nbsp;&nbsp; **delete:notification** | Grants read/write/delete access to user notifications. Currently the same as `write:notification`. |
| **organization** | `orgs/*` and `teams/*` API routes: Organization and team management operations. |
| &nbsp;&nbsp;&nbsp; **read:organization** | Grants read access to org and team status, such as listing all orgs a user has visibility to, teams, and team members. |
| &nbsp;&nbsp;&nbsp; **write:organization** | Grants read/write access to org and team status, such as creating and updating teams and updating org settings. |
| &nbsp;&nbsp;&nbsp; **delete:organization** | Grants read/write/delete access to org and team status, such as deleting teams and orgs. |
| **package** | `/packages/*` API routes: Packages operations |
| &nbsp;&nbsp;&nbsp; **read:package** | Grants read access to package operations, such as reading and downloading available packages. |
| &nbsp;&nbsp;&nbsp; **write:package** | Grants read/write access to package operations. Currently the same as `read:package`. |
| &nbsp;&nbsp;&nbsp; **delete:package** | Grants read/write/delete access to package operations, such as deleting packages. |
| **repository** | `/repos/*` API routes except `/repos/issues/*`: Repository file, pull-request, and release operations. |
| &nbsp;&nbsp;&nbsp; **read:repository** | Grants read access to repository operations, such as getting repository files, releases, collaborators. |
| &nbsp;&nbsp;&nbsp; **write:repository** | Grants read/write access to repository operations, such as getting updating repository files, creating pull requests, updating collaborators. |
| &nbsp;&nbsp;&nbsp; **delete:repository** | Grants read/write/delete access to repository operations, such as getting deleting repository file, delete pull-request, removing collaborators. |
| **user** | `/user/*` and `/users/*` API routes: User-related operations. |
| &nbsp;&nbsp;&nbsp; **read:user** | Grants read access to user operations, such as getting user repo subscriptions and user settings. |
| &nbsp;&nbsp;&nbsp; **write:user** | Grants read/write access to user operations, such as updating user repo subscriptions, followed users, and user settings. |
| &nbsp;&nbsp;&nbsp; **delete:user** | Grants read/write/delete access to user operations, such as removing user repo subscriptions. |

## Client types

Gitea supports both confidential and public client types, [as defined by RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749#section-2.1).

For public clients, a redirect URI of a loopback IP address such as `http://127.0.0.1/` allows any port. Avoid using `localhost`, [as recommended by RFC 8252](https://datatracker.ietf.org/doc/html/rfc8252#section-8.3).

## Example

**Note:** This example does not use PKCE.

1. Redirect to user to the authorization endpoint in order to get their consent for accessing the resources:

   ```curl
   https://[YOUR-GITEA-URL]/login/oauth/authorize?client_id=CLIENT_ID&redirect_uri=REDIRECT_URI&response_type=code&state=STATE
   ```

   The `CLIENT_ID` can be obtained by registering an application in the settings. The `STATE` is a random string that will be send back to your application after the user authorizes. The `state` parameter is optional but should be used to prevent CSRF attacks.

   ![Authorization Page](/authorize.png)

   The user will now be asked to authorize your application. If they authorize it, the user will be redirected to the `REDIRECT_URL`, for example:

   ```curl
   https://[REDIRECT_URI]?code=RETURNED_CODE&state=STATE
   ```

2. Using the provided `code` from the redirect, you can request a new application and refresh token. The access token endpoints accepts POST requests with `application/json` and `application/x-www-form-urlencoded` body, for example:

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

   The `CLIENT_SECRET` is the unique secret code generated for this application. Please note that the secret will only be visible after you created/registered the application with Gitea and cannot be recovered. If you lose the secret you must regenerate the secret via the application's settings.

   The `REDIRECT_URI` in the `access_token` request must match the `REDIRECT_URI` in the `authorize` request.

3. Use the `access_token` to make [API requests](https://docs.gitea.io/en-us/api-usage#oauth2) to access the user's resources.
