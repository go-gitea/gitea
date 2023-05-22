---
date: "2019-09-06T01:35:00-03:00"
title: "Repository indexer"
slug: "repo-indexer"
weight: 45
toc: false
draft: false
aliases:
  - /en-us/repo-indexer
menu:
  sidebar:
    parent: "administration"
    name: "Repository indexer"
    weight: 45
    identifier: "repo-indexer"
---

# Repository indexer

**Table of Contents**

{{< toc >}}

## Setting up the repository indexer

Gitea can search through the files of the repositories by enabling this function in your [`app.ini`](https://docs.gitea.io/en-us/config-cheat-sheet/):

```ini
[indexer]
; ...
REPO_INDEXER_ENABLED = true
REPO_INDEXER_PATH = indexers/repos.bleve
MAX_FILE_SIZE = 1048576
REPO_INDEXER_INCLUDE =
REPO_INDEXER_EXCLUDE = resources/bin/**
```

Please bear in mind that indexing the contents can consume a lot of system resources, especially when the index is created for the first time or globally updated (e.g. after upgrading Gitea).

### Choosing the files for indexing by size

The `MAX_FILE_SIZE` option will make the indexer skip all files larger than the specified value.

### Choosing the files for indexing by path

Gitea applies glob pattern matching from the [`gobwas/glob` library](https://github.com/gobwas/glob) to choose which files will be included in the index.

Limiting the list of files prevents the indexes from becoming polluted with derived or irrelevant files (e.g. lss, sym, map, etc.), so the search results are more relevant. It can also help reduce the index size.

`REPO_INDEXER_EXCLUDE_VENDORED` (default: true) excludes vendored files from index.

`REPO_INDEXER_INCLUDE` (default: empty) is a comma separated list of glob patterns to **include** in the index. An empty list means "_include all files_".
`REPO_INDEXER_EXCLUDE` (default: empty) is a comma separated list of glob patterns to **exclude** from the index. Files that match this list will not be indexed. `REPO_INDEXER_EXCLUDE` takes precedence over `REPO_INDEXER_INCLUDE`.

Pattern matching works as follows:

- To match all files with a `.txt` extension no matter what directory, use `**.txt`.
- To match all files with a `.txt` extension _only at the root level of the repository_, use `*.txt`.
- To match all files inside `resources/bin` and below, use `resources/bin/**`.
- To match all files _immediately inside_ `resources/bin`, use `resources/bin/*`.
- To match all files named `Makefile`, use `**Makefile`.
- Matching a directory has no effect; the pattern `resources/bin` will not include/exclude files inside that directory; `resources/bin/**` will.
- All files and patterns are normalized to lower case, so `**Makefile`, `**makefile` and `**MAKEFILE` are equivalent.
