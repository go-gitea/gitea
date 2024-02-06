---
date: "2024-01-31T00:00:00+00:00"
title: "Blocking a user"
slug: "blocking-user"
sidebar_position: 25
toc: false
draft: false
aliases:
  - /en-us/webhooks
menu:
  sidebar:
    parent: "usage"
    name: "Blocking a user"
    sidebar_position: 30
    identifier: "blocking-user"
---

# Blocking a user

Gitea supports blocking of users to restrict how they can interact with you and your content.

You can block a user in your account settings, from the user's profile or from comments created by the user.
The user is not directly notified about the block, but they can notice they are blocked when they attempt to interact with you.
Organization owners can block anyone who is not a member of the organization too.
If a blocked user has admin permissions, they can still perform all actions even if blocked.

### When you block a user

- the user stops following you
- you stop following the user
- the user's stars are removed from your repositories
- your stars are removed from their repositories
- the user stops watching your repositories
- you stop watching their repositories
- the user's issue assignments are removed from your repositories
- your issue assignments are removed from their repositories
- the user is removed as a collaborator on your repositories
- you are removed as a collaborator on their repositories
- any pending repository transfers to or from the blocked user are canceled

### When you block a user, the user cannot

- follow you
- watch your repositories
- star your repositories
- fork your repositories
- transfer repositories to you
- open issues or pull requests on your repositories
- comment on issues or pull requests you've created
- comment on issues or pull requests on your repositories
- react to your comments on issues or pull requests
- react to comments on issues or pull requests on your repositories
- assign you to issues or pull requests
- add you as a collaborator on their repositories
- send you notifications by @mentioning your username
- be added as team member (if blocked by an organization)
