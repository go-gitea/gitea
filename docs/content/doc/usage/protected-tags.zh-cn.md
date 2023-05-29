---
date: "2023-05-23T09:00:00+08:00"
title: "受保护的标签"
slug: "protected-tags"
weight: 45
toc: false
draft: false
aliases:
  - /zh-cn/protected-tags
menu:
  sidebar:
    parent: "usage"
    name: "受保护的标签"
    weight: 45
    identifier: "protected-tags"
---

# 受保护的标签

受保护的标签允许控制谁有权限创建或更新 Git 标签。每个规则可以匹配单个标签名称，或者使用适当的模式来同时控制多个标签。

**目录**

{{< toc >}}

## 设置受保护的标签

要保护一个标签，你需要按照以下步骤进行操作：

1. 进入仓库的**设置** > **标签**页面。
2. 输入一个用于匹配名称的模式。你可以使用单个名称、[glob 模式](https://pkg.go.dev/github.com/gobwas/glob#Compile) 或正则表达式。
3. 选择允许的用户和/或团队。如果将这些字段留空，则不允许任何人创建或修改此标签。
4. 选择**保存**以保存配置。

## 模式受保护的标签

该模式使用 [glob](https://pkg.go.dev/github.com/gobwas/glob#Compile) 或正则表达式来匹配标签名称。对于正则表达式，你需要将模式括在斜杠中。

示例：

| 类型  | 模式受保护的标签    | 可能匹配的标签                    |
| ----- | ------------------------ | --------------------------------------- |
| Glob  | `v*`                     | `v`，`v-1`，`version2`                  |
| Glob  | `v[0-9]`                 | `v0`，`v1` 到 `v9`                   |
| Glob  | `*-release`              | `2.1-release`，`final-release`          |
| Glob  | `gitea`                  | 仅限 `gitea`                            |
| Glob  | `*gitea*`                | `gitea`，`2.1-gitea`，`1_gitea-release` |
| Glob  | `{v,rel}-*`              | `v-`，`v-1`，`v-final`，`rel-`，`rel-x` |
| Glob  | `*`                      | 匹配所有可能的标签名称          |
| Regex | `/\Av/`                  | `v`，`v-1`，`version2`                  |
| Regex | `/\Av[0-9]\z/`           | `v0`，`v1` 到 `v9`                   |
| Regex | `/\Av\d+\.\d+\.\d+\z/`   | `v1.0.17`，`v2.1.0`                     |
| Regex | `/\Av\d+(\.\d+){0,2}\z/` | `v1`，`v2.1`，`v1.2.34`                 |
| Regex | `/-release\z/`           | `2.1-release`，`final-release`          |
| Regex | `/gitea/`                | `gitea`，`2.1-gitea`，`1_gitea-release` |
| Regex | `/\Agitea\z/`            | 仅限 `gitea`                            |
| Regex | `/^gitea$/`              | 仅限 `gitea`                            |
| Regex | `/\A(v\|rel)-/`          | `v-`，`v-1`，`v-final`，`rel-`，`rel-x` |
| Regex | `/.+/`                   | 匹配所有可能的标签名称          |
