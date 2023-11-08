---
date: "2023-05-24T15:00:00+08:00"
title: "与GitHub Actions的对比"
slug: "comparison"
sidebar_position: 30
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "对比"
    sidebar_position: 30
    identifier: "actions-comparison"
---

# 与GitHub Actions的对比

尽管Gitea Actions旨在与GitHub Actions兼容，但它们之间存在一些差异。

## 额外功能

### Action URL绝对路径

Gitea Actions支持通过URL绝对路径定义actions，这意味着您可以使用来自任何Git存储库的Actions。
例如，`uses: https://github.com/actions/checkout@v3`或`uses: http://your_gitea.com/owner/repo@branch`。

### 使用Go编写Actions

Gitea Actions支持使用Go编写Actions。
请参阅[创建Go Actions](https://blog.gitea.com/creating-go-actions/)。

## 不支持的工作流语法

### `concurrency`

这是用于一次运行一个Job。
请参阅[使用并发](https://docs.github.com/zh/actions/using-jobs/using-concurrency)。

Gitea Actions目前不支持此功能。

### `run-name`

这是工作流生成的工作流运行的名称。
请参阅[GitHub Actions 的工作流语法](https://docs.github.com/zh/actions/using-workflows/workflow-syntax-for-github-actions#run-name)。

Gitea Actions目前不支持此功能。

### `permissions`和`jobs.<job_id>.permissions`

请参阅[GitHub Actions的工作流语法](https://docs.github.com/zh/actions/using-workflows/workflow-syntax-for-github-actions#permissions)。

Gitea Actions目前不支持此功能。

### `jobs.<job_id>.timeout-minutes`

请参阅[GitHub Actions的工作流语法](https://docs.github.com/zh/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idtimeout-minutes)。

Gitea Actions目前不支持此功能。

### `jobs.<job_id>.continue-on-error`

请参阅[GitHub Actions的工作流语法](https://docs.github.com/zh/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idcontinue-on-error)。

Gitea Actions目前不支持此功能。

### `jobs.<job_id>.environment`

请参阅[GitHub Actions的工作流语法](https://docs.github.com/zh/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idenvironment)。

Gitea Actions 目前不支持此功能。

### 复杂的`runs-on`

请参阅[GitHub Actions的工作流语法](https://docs.github.com/zh/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idruns-on)。

Gitea Actions目前只支持`runs-on: xyz`或`runs-on: [xyz]`。

### `workflow_dispatch`

请参阅[GitHub Actions的工作流语法](https://docs.github.com/zh/actions/using-workflows/workflow-syntax-for-github-actions#onworkflow_dispatch)。

Gitea Actions目前不支持此功能。

### `hashFiles`表达式

请参阅[表达式](https://docs.github.com/en/actions/learn-github-actions/expressions#hashfiles)。

Gitea Actions目前不支持此功能，如果使用它，结果将始终为空字符串。

作为解决方法，您可以使用[go-hashfiles](https://gitea.com/actions/go-hashfiles)。

## 缺失的功能

### 变量

请参阅[变量](https://docs.github.com/zh/actions/learn-github-actions/variables)。

目前变量功能正在开发中。

### 问题匹配器

问题匹配器是一种扫描Actions输出以查找指定正则表达式模式并在用户界面中突出显示该信息的方法。
请参阅[问题匹配器](https://github.com/actions/toolkit/blob/main/docs/problem-matchers.md)。

Gitea Actions目前不支持此功能。

### 为错误创建注释

请参阅[为错误创建注释](https://docs.github.com/zh/actions/using-workflows/workflow-commands-for-github-actions#example-creating-an-annotation-for-an-error)。

Gitea Actions目前不支持此功能。

## 缺失的UI功能

### 预处理和后处理步骤

预处理和后处理步骤在Job日志用户界面中没有自己的用户界面。

## 不一样的行为

### 下载Actions

当 `[actions].DEFAULT_ACTIONS_URL` 保持默认值为 `github` 时，Gitea将会从 https://github.com 下载相对路径的actions。比如：
如果你使用 `uses: actions/checkout@v3`，Gitea将会从 https://github.com/actions/checkout.git 下载这个 actions 项目。
如果你想要从另外一个 Git服务下载actions，你只需要使用绝对URL `uses: https://gitea.com/actions/checkout@v3` 来下载。

如果你的 Gitea 实例是部署在一个互联网限制的网络中，有可以使用绝对地址来下载 actions。你也可以讲配置项修改为 `[actions].DEFAULT_ACTIONS_URL = self`。这样所有的相对路径的actions引用，将不再会从 github.com 去下载，而会从这个 Gitea 实例自己的仓库中去下载。例如： `uses: actions/checkout@v3` 将会从 `[server].ROOT_URL`/actions/checkout.git 这个地址去下载 actions。

设置`[actions].DEFAULT_ACTIONS_URL`进行配置。请参阅[配置备忘单](administration/config-cheat-sheet.md#actions-actions)。

### 上下文可用性

不检查上下文可用性，因此您可以在更多地方使用env上下文。
请参阅[上下文可用性](https://docs.github.com/en/actions/learn-github-actions/contexts#context-availability)。
