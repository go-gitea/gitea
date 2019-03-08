---
date: "2016-12-01T16:00:00+02:00"
title: "Webhooks"
slug: "webhooks"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Webhooks"
    weight: 30
    identifier: "webhooks"
---

# Webhooks

Gitea supports web hooks for repository events. This can be found in the settings
page `/:username/:reponame/settings/hooks`. All event pushes are POST requests.
The two methods currently supported are Gitea and Slack.

### Event information

The following is an example of event information that will be sent by Gitea to
a Payload URL:


```
X-GitHub-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-GitHub-Event: push
X-Gogs-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-Gogs-Event: push
X-Gitea-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-Gitea-Event: push
```

```json
{
  "secret": "3gEsCfjlV2ugRwgpU#w1*WaW*wa4NXgGmpCfkbG3",
  "ref": "refs/heads/develop",
  "before": "28e1879d029cb852e4844d9c718537df08844e03",
  "after": "bffeb74224043ba2feb48d137756c8a9331c449a",
  "compare_url": "http://localhost:3000/gitea/webhooks/compare/28e1879d029cb852e4844d9c718537df08844e03...bffeb74224043ba2feb48d137756c8a9331c449a",
  "commits": [
    {
      "id": "bffeb74224043ba2feb48d137756c8a9331c449a",
      "message": "Webhooks Yay!",
      "url": "http://localhost:3000/gitea/webhooks/commit/bffeb74224043ba2feb48d137756c8a9331c449a",
      "author": {
        "name": "Gitea",
        "email": "someone@gitea.io",
        "username": "gitea"
      },
      "committer": {
        "name": "Gitea",
        "email": "someone@gitea.io",
        "username": "gitea"
      },
      "timestamp": "2017-03-13T13:52:11-04:00"
    }
  ],
  "repository": {
    "id": 140,
    "owner": {
      "id": 1,
      "login": "gitea",
      "full_name": "Gitea",
      "email": "someone@gitea.io",
      "avatar_url": "https://localhost:3000/avatars/1",
      "username": "gitea"
    },
    "name": "webhooks",
    "full_name": "gitea/webhooks",
    "description": "",
    "private": false,
    "fork": false,
    "html_url": "http://localhost:3000/gitea/webhooks",
    "ssh_url": "ssh://gitea@localhost:2222/gitea/webhooks.git",
    "clone_url": "http://localhost:3000/gitea/webhooks.git",
    "website": "",
    "stars_count": 0,
    "forks_count": 1,
    "watchers_count": 1,
    "open_issues_count": 7,
    "default_branch": "master",
    "created_at": "2017-02-26T04:29:06-05:00",
    "updated_at": "2017-03-13T13:51:58-04:00"
  },
  "pusher": {
    "id": 1,
    "login": "gitea",
    "full_name": "Gitea",
    "email": "someone@gitea.io",
    "avatar_url": "https://localhost:3000/avatars/1",
    "username": "gitea"
  },
  "sender": {
    "id": 1,
    "login": "gitea",
    "full_name": "Gitea",
    "email": "someone@gitea.io",
    "avatar_url": "https://localhost:3000/avatars/1",
    "username": "gitea"
  }
}
```
