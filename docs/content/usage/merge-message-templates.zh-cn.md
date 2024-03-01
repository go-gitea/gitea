---
date: "2023-05-23T09:00:00+08:00"
title: "合并消息模板"
slug: "merge-message-templates"
sidebar_position: 15
toc: false
draft: false
aliases:
  - /zh-cn/merge-message-templates
menu:
  sidebar:
    parent: "usage"
    name: "合并消息模板"
    sidebar_position: 15
    identifier: "merge-message-templates"
---

# 合并消息模板

## 文件名

PR 默认合并消息模板可能的文件名：

- `.gitea/default_merge_message/MERGE_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE-MERGE_TEMPLATE.md`
- `.gitea/default_merge_message/SQUASH_TEMPLATE.md`
- `.gitea/default_merge_message/MANUALLY-MERGED_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE-UPDATE-ONLY_TEMPLATE.md`

## 变量

您可以在这些模板中使用以下以 `${}` 包围的变量，这些变量遵循 [os.Expand](https://pkg.go.dev/os#Expand) 语法：

- BaseRepoOwnerName：此合并请求的基础仓库所有者名称
- BaseRepoName：此合并请求的基础仓库名称
- BaseBranch：此合并请求的基础仓库目标分支名称
- HeadRepoOwnerName：此合并请求的源仓库所有者名称
- HeadRepoName：此合并请求的源仓库名称
- HeadBranch：此合并请求的源仓库分支名称
- PullRequestTitle：合并请求的标题
- PullRequestDescription：合并请求的描述
- PullRequestPosterName：合并请求的提交者名称
- PullRequestIndex：合并请求的索引号
- PullRequestReference：合并请求的引用字符与索引号。例如，#1、!2
- ClosingIssues：返回一个包含将由此合并请求关闭的所有工单的字符串。例如 `close #1, close #2`

## 变基（Rebase）

在没有合并提交的情况下进行变基时，`REBASE_TEMPLATE.md` 修改最后一次提交的消息。此模板还提供以下附加变量：

- CommitTitle：提交的标题
- CommitBody：提交的正文文本
