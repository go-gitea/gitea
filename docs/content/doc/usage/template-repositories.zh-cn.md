---
date: "2023-05-23T09:00:00+08:00"
title: "模板仓库"
slug: "template-repositories"
weight: 14
toc: false
draft: false
aliases:
  - /zh-cn/template-repositories
menu:
  sidebar:
    parent: "usage"
    name: "模板仓库"
    weight: 14
    identifier: "template-repositories"
---

# 模板仓库

**目录**

{{< toc >}}

Gitea `1.11.0` 及以上版本引入了模板仓库，并且其中一个实现的功能是自动展开模板文件中的特定变量。

要告诉 Gitea 哪些文件需要展开，您必须在模板仓库的 `.gitea` 目录中包含一个 `template` 文件。

Gitea 使用 [gobwas/glob](https://github.com/gobwas/glob) 作为其 glob 语法。它与传统的 `.gitignore` 语法非常相似，但可能存在细微的差异。

## `.gitea/template` 文件示例

所有路径都是相对于仓库的根目录

```gitignore
# 仓库中的所有 .go 文件
**.go

# text 目录中的所有文本文件
text/*.txt

# 特定文件
a/b/c/d.json

# 匹配批处理文件的大小写变体
**.[bB][aA][tT]
```

**注意：** 当从模板生成仓库时，`.gitea` 目录中的 `template` 文件将被删除。

## 参数展开

在与上述通配符匹配的任何文件中，将会扩展某些变量。

所有变量都必须采用`$VAR`或`${VAR}`的形式。要转义扩展，使用双重`$$`，例如`$$VAR`或`$${VAR}`。

| 变量                  | 扩展为                                               | 可转换       |
| -------------------- | --------------------------------------------------- | ------------- |
| REPO_NAME            | 生成的仓库名称                                       | ✓             |
| TEMPLATE_NAME        | 模板仓库名称                                         | ✓             |
| REPO_DESCRIPTION     | 生成的仓库描述                                       | ✘             |
| TEMPLATE_DESCRIPTION | 模板仓库描述                                         | ✘             |
| REPO_OWNER           | 生成的仓库所有者                                     | ✓             |
| TEMPLATE_OWNER       | 模板仓库所有者                                       | ✓             |
| REPO_LINK            | 生成的仓库链接                                       | ✘             |
| TEMPLATE_LINK        | 模板仓库链接                                         | ✘             |
| REPO_HTTPS_URL       | 生成的仓库的 HTTP(S) 克隆链接                         | ✘             |
| TEMPLATE_HTTPS_URL   | 模板仓库的 HTTP(S) 克隆链接                           | ✘             |
| REPO_SSH_URL         | 生成的仓库的 SSH 克隆链接                             | ✘             |
| TEMPLATE_SSH_URL     | 模板仓库的 SSH 克隆链接                               | ✘             |

## 转换器 :robot:

Gitea `1.12.0` 添加了一些转换器以应用于上述适用的变量。

例如，要以 `PASCAL`-case 获取 `REPO_NAME`，你的模板应使用 `${REPO_NAME_PASCAL}`

将 `go-sdk` 传递给可用的转换器的效果如下...

| 转换器      | 效果         |
| ----------- | ------------ |
| SNAKE       | go_sdk       |
| KEBAB       | go-sdk       |
| CAMEL       | goSdk        |
| PASCAL      | GoSdk        |
| LOWER       | go-sdk       |
| UPPER       | GO-SDK       |
| TITLE       | Go-Sdk       |
