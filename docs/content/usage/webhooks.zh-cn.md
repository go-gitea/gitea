---
date: "2016-12-01T16:00:00+02:00"
title: "Webhooks"
slug: "webhooks"
sidebar_position: 30
toc: false
draft: false
aliases:
  - /zh-cn/webhooks
menu:
  sidebar:
    parent: "usage"
    name: "Webhooks"
    sidebar_position: 30
    identifier: "webhooks"
---

# Webhooks

Gitea支持用于仓库事件的Webhooks。这可以在仓库管理员在设置页面 `/:username/:reponame/settings/hooks` 中进行配置。Webhooks还可以基于组织和整个系统进行配置。
所有事件推送都是 POST 请求。目前支持：

- Gitea (也可以是 GET 请求)
- Gogs
- Slack
- Discord
- Dingtalk（钉钉）
- Telegram
- Microsoft Teams
- Feishu
- Wechatwork（企业微信）
- Packagist

### 事件信息

**警告**：自 Gitea 1.13.0 版起，payload 中的 `secret` 字段已被弃用，并将在 1.14.0 版中移除：https://github.com/go-gitea/gitea/issues/11755

以下是 Gitea 将发送给 payload URL的事件信息示例：

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

### 示例

这是一个示例，演示如何使用 Webhooks 在推送请求到达仓库时运行一个 php 脚本。
在你的仓库设置中，在 Webhooks 下，设置一个如下的 Gitea webhook：

- 目标 URL：http://mydomain.com/webhook.php
- HTTP 方法：POST
- POST Content Type：application/json
- Secret：123
- 触发条件：推送事件
- 激活：勾选

现在在你的服务器上创建 php 文件 webhook.php。

```
<?php

$secret_key = '123';

// check for POST request
if ($_SERVER['REQUEST_METHOD'] != 'POST') {
    error_log('FAILED - not POST - '. $_SERVER['REQUEST_METHOD']);
    exit();
}

// get content type
$content_type = isset($_SERVER['CONTENT_TYPE']) ? strtolower(trim($_SERVER['CONTENT_TYPE'])) : '';

if ($content_type != 'application/json') {
    error_log('FAILED - not application/json - '. $content_type);
    exit();
}

// get payload
$payload = trim(file_get_contents("php://input"));

if (empty($payload)) {
    error_log('FAILED - no payload');
    exit();
}

// get header signature
$header_signature = isset($_SERVER['HTTP_X_GITEA_SIGNATURE']) ? $_SERVER['HTTP_X_GITEA_SIGNATURE'] : '';

if (empty($header_signature)) {
    error_log('FAILED - header signature missing');
    exit();
}

// calculate payload signature
$payload_signature = hash_hmac('sha256', $payload, $secret_key, false);

// check payload signature against header signature
if ($header_signature !== $payload_signature) {
    error_log('FAILED - payload signature');
    exit();
}

// convert json to array
$decoded = json_decode($payload, true);

// check for json decode errors
if (json_last_error() !== JSON_ERROR_NONE) {
    error_log('FAILED - json decode - '. json_last_error());
    exit();
}

// success, do something
```

在 Webhook 设置中有一个“测试推送（Test Delivery）”按钮，可以测试配置，还有一个“最近推送记录（Recent Deliveries）”的列表。

### 授权头（Authorization header）

**从1.19版本开始**，Gitea 的 Webhook 可以配置为向 Webhook 目标发送一个 [授权头（authorization header）](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Authorization)。
