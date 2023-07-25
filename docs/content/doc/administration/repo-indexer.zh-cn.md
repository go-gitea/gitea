---
date: "2023-05-23T09:00:00+08:00"
title: "仓库索引器"
slug: "repo-indexer"
weight: 45
toc: false
draft: false
aliases:
  - /zh-cn/repo-indexer
menu:
  sidebar:
    parent: "administration"
    name: "仓库索引器"
    weight: 45
    identifier: "repo-indexer"
---

# 仓库索引器

**目录**

{{< toc >}}

## 设置仓库索引器

通过在您的 [`app.ini`](https://docs.gitea.io/en-us/config-cheat-sheet/) 中启用此功能，Gitea 可以通过仓库的文件进行搜索：

```ini
[indexer]
; ...
REPO_INDEXER_ENABLED = true
REPO_INDEXER_PATH = indexers/repos.bleve
MAX_FILE_SIZE = 1048576
REPO_INDEXER_INCLUDE =
REPO_INDEXER_EXCLUDE = resources/bin/**
```

请记住，索引内容可能会消耗大量系统资源，特别是在首次创建索引或全局更新索引时（例如升级 Gitea 之后）。

### 按大小选择要索引的文件

`MAX_FILE_SIZE` 选项将使索引器跳过所有大于指定值的文件。

### 按路径选择要索引的文件

Gitea使用 [`gobwas/glob` 库](https://github.com/gobwas/glob) 中的 glob 模式匹配来选择要包含在索引中的文件。

限制文件列表可以防止索引被派生或无关的文件（例如 lss、sym、map 等）污染，从而使搜索结果更相关。这还有助于减小索引的大小。

`REPO_INDEXER_EXCLUDE_VENDORED`（默认值为 true）将排除供应商文件不包含在索引中。

`REPO_INDEXER_INCLUDE`（默认值为空）是一个逗号分隔的 glob 模式列表，用于在索引中**包含**的文件。空列表表示“_包含所有文件_”。
`REPO_INDEXER_EXCLUDE`（默认值为空）是一个逗号分隔的 glob 模式列表，用于从索引中**排除**的文件。与该列表匹配的文件将不会被索引。`REPO_INDEXER_EXCLUDE` 优先于 `REPO_INDEXER_INCLUDE`。

模式匹配工作方式如下：

- 要匹配所有带有 `.txt` 扩展名的文件，无论在哪个目录中，请使用 `**.txt`。
- 要匹配仅在仓库的根级别中具有 `.txt` 扩展名的所有文件，请使用 `*.txt`。
- 要匹配 `resources/bin` 目录及其子目录中的所有文件，请使用 `resources/bin/**`。
- 要匹配位于 `resources/bin` 目录下的所有文件，请使用 `resources/bin/*`。
- 要匹配所有名为 `Makefile` 的文件，请使用 `**Makefile`。
- 匹配目录没有效果；模式 `resources/bin` 不会包含/排除该目录中的文件；`resources/bin/**` 会。
- 所有文件和模式都规范化为小写，因此 `**Makefile`、`**makefile` 和 `**MAKEFILE` 是等效的。
