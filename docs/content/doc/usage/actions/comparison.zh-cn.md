---
date: "2023-05-24T15:00:00+08:00"
title: "与GitHub Actions的对比"
slug: "comparison"
weight: 30
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "对比"
    weight: 30
    identifier: "actions-comparison"
---

# 与GitHub Actions的对比

尽管Gitea Actions旨在与GitHub Actions兼容，但它们之间存在一些差异。

**目录**

{{< toc >}}

## 额外功能

### Action URL绝对路径

Gitea Actions支持通过URL绝对路径定义actions，这意味着您可以使用来自任何Git存储库的Actions。
例如，`uses: https://github.com/actions/checkout@v3`或`uses: http://your_gitea.com/owner/repo@branch`。

### 使用Go编写Actions

Gitea Actions支持使用Go编写Actions。
请参阅[创建Go Actions](https://blog.gitea.io/2023/04/creating-go-actions/)。

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

Gitea Actions默认不从GitHub下载Actions。
"默认" 意味着您在`uses` 字段中不指定主机，如`uses: actions/checkout@v3`。
相反，`uses: https://github.com/actions/checkout@v3`是有指定主机的。

如果您不进行配置，缺失的主机将填充为`https://gitea.com`。
这意味着`uses: actions/checkout@v3`将从[gitea.com/actions/checkout](https://gitea.com/actions/checkout)下载该Action，而不是[github.com/actions/checkout](https://github.com/actions/checkout)。

正如前面提到的，这是可配置的。
如果您希望您的运行程序默认从GitHub或您自己的Gitea实例下载动作，您可以通过设置`[actions].DEFAULT_ACTIONS_URL`进行配置。请参阅[配置备忘单]({{< relref "doc/administration/config-cheat-sheet.zh-cn.md#actions-actions" >}})。

### 上下文可用性

不检查上下文可用性，因此您可以在更多地方使用env上下文。
请参阅[上下文可用性](https://docs.github.com/en/actions/learn-github-actions/contexts#context-availability)。

## 已知问题

### `docker/build-push-action@v4`

请参阅[act_runner#119](https://gitea.com/gitea/act_runner/issues/119#issuecomment-738294)。

`ACTIONS_RUNTIME_TOKEN`在Gitea Actions中是一个随机字符串，而不是JWT。
但是`DOCKER/BUILD-PUSH-ACTION@V4尝试将令牌解析为JWT，并且不处理错误，因此Job失败。

有两种解决方法：

手动将`ACTIONS_RUNTIME_TOKEN`设置为空字符串，例如：

``` yml
- name: Build and push
  uses: docker/build-push-action@v4
  env:
    ACTIONS_RUNTIME_TOKEN: ''
  with:
...
```

该问题已在较新的[提交](https://gitea.com/docker/build-push-action/commit/d8823bfaed2a82c6f5d4799a2f8e86173c461aba?style=split&whitespace=show-all#diff-1af9a5bdf96ddff3a2f3427ed520b7005e9564ad)中修复，但尚未发布。因此，您可以通过指定分支名称来使用最新版本，例如：

``` yml
- name: Build and push
  uses: docker/build-push-action@master
  with:
...
```
