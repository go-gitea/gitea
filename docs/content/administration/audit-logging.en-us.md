---
date: "2023-04-21T00:00:00+00:00"
title: "Audit Logging"
slug: "audit-logging"
sidebar_position: 43
toc: false
draft: false
menu:
  sidebar:
    parent: "administration"
    name: "Audit Logging"
    sidebar_position: 43
    identifier: "audit-logging"
---

# Audit Logging

Audit logging is used to track security related events and provide documentary evidence of the sequence of important activities.

**Table of Contents**

{{< toc >}}

## Appenders

The audit log supports different appenders:

- `log`: Log events as information to the configured Gitea logging
- `file`: Write events as JSON objects to a file

The config documentation lists all available options to configure audit logging with appenders.

## Events

Audit events are grouped by `user`, `organization`, `repository` and `system`.

### User Events

| Event | Description |
| - | - |
| `user:impersonation` | Admin impersonating user |
| `user:create` | Created user |
| `user:delete` | Deleted user |
| `user:authentication:fail:twofactor` | Failed two-factor authentication for user |
| `user:authentication:source` | Changed authentication source of user |
| `user:active` | Changed activation status of user |
| `user:restricted` | Changed restriction status of user |
| `user:admin` | Changed admin status of user |
| `user:name` | Changed user name |
| `user:password` | Changed password of user |
| `user:password:resetrequest` | Requested a password reset |
| `user:visibility` | Changed visibility of user |
| `user:email:primary` | Changed primary email of user |
| `user:email:add` | Added email to user |
| `user:email:activate` | Activated email of user |
| `user:email:remove` | Removed email from user |
| `user:twofactor:enable` | User enabled two-factor authentication |
| `user:twofactor:regenerate` | User regenerated two-factor authentication secret |
| `user:twofactor:disable` | User disabled two-factor authentication |
| `user:webauth:add` | User added WebAuthn key |
| `user:webauth:remove` | User removed WebAuthn key |
| `user:externallogin:add` | Added external login for user |
| `user:externallogin:remove` | Removed external login for user |
| `user:openid:add` | Associated OpenID to user |
| `user:openid:remove` | Removed OpenID from user |
| `user:accesstoken:add` | Added access token for user |
| `user:accesstoken:remove` | Removed access token from user |
| `user:oauth2application:add` | Created OAuth2 application |
| `user:oauth2application:update` | Updated OAuth2 application |
| `user:oauth2application:secret` | Regenerated secret for OAuth2 application |
| `user:oauth2application:grant` | Granted OAuth2 access to application |
| `user:oauth2application:revoke` | Revoked OAuth2 grant for application |
| `user:oauth2application:remove` | Removed OAuth2 application |
| `user:key:ssh:add` | Added SSH key |
| `user:key:ssh:remove` | Removed SSH key |
| `user:key:principal:add` | Added principal key |
| `user:key:principal:remove` | Removed principal key |
| `user:key:gpg:add` | Added GPG key |
| `user:key:gpg:remove` | Added GPG key |
| `user:secret:add` | Added secret |
| `user:secret:update` | Updated secret |
| `user:secret:remove` | Removed secret |
| `user:webhook:add` | Added webhook |
| `user:webhook:update` | Updated webhook |
| `user:webhook:remove` | Removed webhook |

### Organization Events

| Event | Description |
| - | - |
| `organization:create` | Created organization |
| `organization:delete` | Deleted organization |
| `organization:name` | Changed organization name |
| `organization:visibility` | Changed visibility of organization |
| `organization:team:add` | Added team to organization |
| `organization:team:update` | Updated settings of team |
| `organization:team:remove` | Removed team from organization |
| `organization:team:permission` | Changed permission of team |
| `organization:team:member:add` | Added user to team |
| `organization:team:member:remove` | Removed User from team |
| `organization:oauth2application:add` | Created OAuth2 application |
| `organization:oauth2application:update` | Updated OAuth2 application |
| `organization:oauth2application:secret` | Regenerated secret for OAuth2 application |
| `organization:oauth2application:remove` | Removed OAuth2 application |
| `organization:secret:add` | Added secret |
| `organization:secret:update` | Updated secret |
| `organization:secret:remove` | Removed secret |
| `organization:webhook:add` | Added webhook |
| `organization:webhook:update` | Updated webhook |
| `organization:webhook:remove` | Removed webhook |

### Repository Events

| Event | Description |
| - | - |
| `repository:create` | Crated repository |
| `repository:create:fork` | Created fork of repository |
| `repository:archive` | Archived repository |
| `repository:unarchive` | Unarchived repository |
| `repository:delete` | Deleted repository |
| `repository:name` | Changed repository name |
| `repository:visibility` | Changed visibility of repository |
| `repository:convert:fork` | Converted repository from fork to regular repository |
| `repository:convert:mirror` | Converted repository from mirror to regular repository |
| `repository:mirror:push:add` | Added push mirror for repository |
| `repository:mirror:push:remove` | Removed push mirror from repository |
| `repository:signingverification` | Changed signing verification of repository |
| `repository:transfer:start` | Started repository transfer |
| `repository:transfer:finish` | Transferred repository to new owner |
| `repository:transfer:cancel` | Canceled repository transfer |
| `repository:wiki:delete` | Deleted wiki of repository |
| `repository:collaborator:add` | Added user as collaborator for repository |
| `repository:collaborator:access` | Changed access mode of collaborator |
| `repository:collaborator:remove` | Removed user as collaborator of repository |
| `repository:collaborator:team:add` | Added team as collaborator for repository |
| `repository:collaborator:team:remove` | Removed team as collaborator of repository |
| `repository:branch:default` | Changed default branch |
| `repository:branch:protection:add` | Added branch protection |
| `repository:branch:protection:update` | Updated branch protection |
| `repository:branch:protection:remove` | Removed branch protection |
| `repository:tag:protection:add` | Added tag protection |
| `repository:tag:protection:update` | Updated tag protection |
| `repository:tag:protection:remove` | Removed tag protection |
| `repository:webhook:add` | Added webhook |
| `repository:webhook:update` | Updated webhook |
| `repository:webhook:remove` | Removed webhook |
| `repository:deploykey:add` | Added deploy key |
| `repository:deploykey:remove` | Removed deploy key |
| `repository:secret:add` | Added secret |
| `repository:secret:update` | Updated secret |
| `repository:secret:remove` | Removed secret |

### System Events

| Event | Description |
| - | - |
| `system:startup` | System startup |
| `system:shutdown` | Normal system shutdown (unexpected shutdowns may not be logged) |
| `system:webhook:add` | Added webhook |
| `system:webhook:update` | Updated webhook |
| `system:webhook:remove` | Removed webhook |
| `system:authenticationsource:add` | Created authentication source |
| `system:authenticationsource:update` | Updated authentication source |
| `system:authenticationsource:remove` | Removed authentication source |
| `system:oauth2application:add` | Created OAuth2 application |
| `system:oauth2application:update` | Updated OAuth2 application |
| `system:oauth2application:secret` | Regenerated secret for OAuth2 application |
| `system:oauth2application:remove` | Removed OAuth2 application |
