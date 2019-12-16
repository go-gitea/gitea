---
date: "2019-11-28:00:00+02:00"
title: "The .gitea Directory"
slug: "gitea-directory"
weight: 40
toc: true
draft: false
menu:
  sidebar:
    parent: "features"
    name: "The .gitea Directory"
    weight: 50
    identifier: "gitea-directory"
---

# The .gitea directory
Gitea repositories can include a `.gitea` directory at their base which will store settings/configurations for certain features.

## Templates
Gitea includes template repositories, and one feature implemented with them is auto-expansion of specific variables within your template files.  
To tell Gitea which files to expand, you must include a `template` file inside the `.gitea` directory of the template repository.  
Gitea uses [gobwas/glob](https://github.com/gobwas/glob) for its glob syntax. It closely resembles a traditional `.gitignore`, however there may be slight differences.

### Example `.gitea/template` file  
All paths are relative to the base of the repository
```gitignore
# All .go files, anywhere in the repository
**.go

# All text files in the text directory
text/*.txt

# A specific file
a/b/c/d.json

# Batch files in both upper or lower case can be matched
**.[bB][aA][tT]
```
**NOTE:** The `template` file will be removed from the `.gitea` directory when a repository is generated from the template.

### Variable Expansion
In any file matched by the above globs, certain variables will be expanded.  
All variables must be of the form `$VAR` or `${VAR}`. To escape an expansion, use a double `$$`, such as `$$VAR` or `$${VAR}`

| Variable             | Expands To                                          |
|----------------------|-----------------------------------------------------|
| REPO_NAME            | The name of the generated repository                |
| TEMPLATE_NAME        | The name of the template repository                 |
| REPO_DESCRIPTION     | The description of the generated repository         |
| TEMPLATE_DESCRIPTION | The description of the template repository          |
| REPO_LINK            | The URL to the generated repository                 |
| TEMPLATE_LINK        | The URL to the template repository                  |
| REPO_HTTPS_URL       | The HTTP(S) clone link for the generated repository |
| TEMPLATE_HTTPS_URL   | The HTTP(S) clone link for the template repository  |
| REPO_SSH_URL         | The SSH clone link for the generated repository     |
| TEMPLATE_SSH_URL     | The SSH clone link for the template repository      |
