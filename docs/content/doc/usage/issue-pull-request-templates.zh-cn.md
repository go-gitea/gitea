---
date: "2018-05-10T16:00:00+02:00"
title: "使用：Issue 和 Pull Request 模板"
slug: "issue-pull-request-templates"
weight: 15
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Issue 和 Pull Request 模板"
    weight: 15
    identifier: "issue-pull-request-templates"
---

# 使用 Issue 和 Pull Request 模板

对于一些项目，在创建 Issue 或 Pull Request 时有一个标准的询问列表需要提交者填写。Gitea 支持添加此类模板至 Repository 的主分支，以便提交者在创建 Issue 或 Pull Request 时可以自动生成一个需要完成的表单，这么做可以减少一些前期关于 Issue 抑或 Pull Request 细节上的沟通成本。

此外，新建 Issue 的页面 URL 支持以 `?title=Issue+Title&body=Issue+Text` 为后缀，表单将使用其中的参数自动填充内容并且覆盖模板。

## 模板文件

以下是受支持的 Issue 模板文件名:

- `ISSUE_TEMPLATE.md`
- `ISSUE_TEMPLATE.yaml`
- `ISSUE_TEMPLATE.yml`
- `issue_template.md`
- `issue_template.yaml`
- `issue_template.yml`
- `.gitea/ISSUE_TEMPLATE.md`
- `.gitea/ISSUE_TEMPLATE.yaml`
- `.gitea/ISSUE_TEMPLATE.yml`
- `.gitea/issue_template.md`
- `.gitea/issue_template.yaml`
- `.gitea/issue_template.md`
- `.github/ISSUE_TEMPLATE.md`
- `.github/ISSUE_TEMPLATE.yaml`
- `.github/ISSUE_TEMPLATE.yml`
- `.github/issue_template.md`
- `.github/issue_template.yaml`
- `.github/issue_template.yml`

以下是受支持的 PR 模板文件名:

- `PULL_REQUEST_TEMPLATE.md`
- `PULL_REQUEST_TEMPLATE.yaml`
- `PULL_REQUEST_TEMPLATE.yml`
- `pull_request_template.md`
- `pull_request_template.yaml`
- `pull_request_template.yml`
- `.gitea/PULL_REQUEST_TEMPLATE.md`
- `.gitea/PULL_REQUEST_TEMPLATE.yaml`
- `.gitea/PULL_REQUEST_TEMPLATE.yml`
- `.gitea/pull_request_template.md`
- `.gitea/pull_request_template.yaml`
- `.gitea/pull_request_template.yml`
- `.github/PULL_REQUEST_TEMPLATE.md`
- `.github/PULL_REQUEST_TEMPLATE.yaml`
- `.github/PULL_REQUEST_TEMPLATE.yml`
- `.github/pull_request_template.md`
- `.github/pull_request_template.yaml`
- `.github/pull_request_template.yml`

## 模板目录

另外，为了便于提问者根据自己的问题类型选择一个恰当的 Issue 模板，仓库所有者可以在模板目录中创建多种类型的 Issue 模板。

以下是受支持的模板目录:

- `ISSUE_TEMPLATE`
- `issue_template`
- `.gitea/ISSUE_TEMPLATE`
- `.gitea/issue_template`
- `.github/ISSUE_TEMPLATE`
- `.github/issue_template`
- `.gitlab/ISSUE_TEMPLATE`
- `.gitlab/issue_template`

目录中可以包含多个 markdown (`.md`) 或 yaml (`.yaml`/`.yml`) 格式的 Issue 模板。

## Markdown 模板语法

```md
---

name: "Template Name"
about: "This template is for testing!"
title: "[TEST] "
ref: "main"
labels:

- bug
- "help needed"

---

This is the template!
```

在上面的示例中，用户将从列表中选择一个 Issue 模板，列表会展示模板名称 `Template Name` 和模板描述 `This template is for testing!`。 同时，标题会预先填充为 `[TEST]`，而正文将预先填充 `This is the template!`。 最后，Issue 还会被分配两个标签，`bug` 和 `help needed`，并且将问题指向 `main` 分支。

## YAML 模板语法

本示例中的 YAML 配置文件定义了一个用于提交 bug 的问卷表单。

```yaml
name: Bug Report
about: File a bug report
title: "[Bug]: "
body:
  - type: markdown
    attributes:
      value: |
        Thanks for taking the time to fill out this bug report!
  - type: input
    id: contact
    attributes:
      label: Contact Details
      description: How can we get in touch with you if we need more info?
      placeholder: ex. email@example.com
    validations:
      required: false
  - type: textarea
    id: what-happened
    attributes:
      label: What happened?
      description: Also tell us, what did you expect to happen?
      placeholder: Tell us what you see!
      value: "A bug happened!"
    validations:
      required: true
  - type: dropdown
    id: version
    attributes:
      label: Version
      description: What version of our software are you running?
      options:
        - 1.0.2 (Default)
        - 1.0.3 (Edge)
    validations:
      required: true
  - type: dropdown
    id: browsers
    attributes:
      label: What browsers are you seeing the problem on?
      multiple: true
      options:
        - Firefox
        - Chrome
        - Safari
        - Microsoft Edge
  - type: textarea
    id: logs
    attributes:
      label: Relevant log output
      description: Please copy and paste any relevant log output. This will be automatically formatted into code, so no need for backticks.
      render: shell
  - type: checkboxes
    id: terms
    attributes:
      label: Code of Conduct
      description: By submitting this issue, you agree to follow our [Code of Conduct](https://example.com)
      options:
        - label: I agree to follow this project's Code of Conduct
          required: true
```

### Markdown

您可以在 YAML 模板中使用 `markdown` 元素为用户提供额外的上下文支撑，这部分并不会作为正文提交。

`attributes:`

| 键      | 描述                           | 必选 | 类型   | 默认值 | 有效值 |
| ------- | ------------------------------ | ---- | ------ | ------ | ------ |
| `value` | 渲染的文本。支持 Markdown 格式 | 必选 | 字符串 | -      | -      |

### Textarea

您可以使用 `textarea` 元素在表单中添加多行文本字段。 贡献者还可以在 `textarea` 字段中附加文件。

`attributes:`

| 键            | 描述                                                                                                  | 必选 | 类型   | 默认值   | 有效值             |
| ------------- | ----------------------------------------------------------------------------------------------------- | ---- | ------ | -------- | ------------------ |
| `label`       | 预期用户输入的简短描述，也以表单形式显示。                                                            | 必选 | 字符串 | -        | -                  |
| `description` | 提供上下文或指导的文本区域的描述，以表单形式显示。                                                    | 可选 | 字符串 | 空字符串 | -                  |
| `placeholder` | 半透明的占位符，在文本区域空白时呈现                                                                  | 可选 | 字符串 | 空字符串 | -                  |
| `value`       | 在文本区域中预填充的文本。                                                                            | 可选 | 字符串 | -        | -                  |
| `render`      | 如果提供了值，提交的文本将格式化为代码块。 提供此键时，文本区域将不会扩展到文件附件或 Markdown 编辑。 | 可选 | 字符串 | -        | Gitea 支持的语言。 |

`validations:`

| 键         | 描述                         | 必选 | 类型   | 默认值 | 有效值 |
| ---------- | ---------------------------- | ---- | ------ | ------ | ------ |
| `required` | 防止在元素完成之前提交表单。 | 可选 | 布尔型 | false  | -      |

### Input

您可以使用 `input` 元素添加单行文本字段到表单。

`attributes:`

| 键            | 描述                                           | 必选 | 类型   | 默认值   | 有效值 |
| ------------- | ---------------------------------------------- | ---- | ------ | -------- | ------ |
| `label`       | 预期用户输入的简短描述，也以表单形式显示。     | 必选 | 字符串 | -        | -      |
| `description` | 提供上下文或指导的字段的描述，以表单形式显示。 | 可选 | 字符串 | 空字符串 | -      |
| `placeholder` | 半透明的占位符，在字段空白时呈现。             | 可选 | 字符串 | 空字符串 | -      |
| `value`       | 字段中预填的文本。                             | 可选 | 字符串 | -        | -      |

`validations:`

| 键          | 描述                             | 必选 | 类型   | 默认值 | 有效值                                                         |
| ----------- | -------------------------------- | ---- | ------ | ------ | -------------------------------------------------------------- |
| `required`  | 防止在未填内容时提交表单。       | 可选 | 布尔型 | false  | -                                                              |
| `is_number` | 防止在未填数字时提交表单。       | 可选 | 布尔型 | false  | -                                                              |
| `regex`     | 直到满足了与正则表达式匹配的值。 | 可选 | 字符串 | -      | [正则表达式](https://en.wikipedia.org/wiki/Regular_expression) |

### Dropdown

您可以使用 `dropdown` 元素在表单中添加下拉菜单。

`attributes:`

| 键            | 描述                                                      | 必选 | 类型       | 默认值   | 有效值 |
| ------------- | --------------------------------------------------------- | ---- | ---------- | -------- | ------ |
| `label`       | 预期用户输入的简短描述，以表单形式显示。                  | 必选 | 字符串     | -        | -      |
| `description` | 提供上下文或指导的下拉列表的描述，以表单形式显示。        | 可选 | 字符串     | 空字符串 | -      |
| `multiple`    | 确定用户是否可以选择多个选项。                            | 可选 | 布尔型     | false    | -      |
| `options`     | 用户可以选择的选项列表。 不能为空，所有选择必须是不同的。 | 必选 | 字符串数组 | -        | -      |

`validations:`

| 键         | 描述                         | 必选 | 类型   | 默认值 | 有效值 |
| ---------- | ---------------------------- | ---- | ------ | ------ | ------ |
| `required` | 防止在元素完成之前提交表单。 | 可选 | 布尔型 | false  | -      |

### Checkboxes

您可以使用 `checkboxes` 元素添加一组复选框到表单。

`attributes:`

| 键            | 描述                                                  | 必选 | 类型   | 默认值   | 有效值 |
| ------------- | ----------------------------------------------------- | ---- | ------ | -------- | ------ |
| `label`       | 预期用户输入的简短描述，以表单形式显示。              | 必选 | 字符串 | -        | -      |
| `description` | 复选框集的描述，以表单形式显示。 支持 Markdown 格式。 | 可选 | 字符串 | 空字符串 | -      |
| `options`     | 用户可以选择的复选框列表。 有关语法，请参阅下文。     | 必选 | 数组   | -        | -      |

对于 `options` 列表中的每个值，您可以设置以下键。

| 键         | 描述                                                                              | 必选 | 类型   | 默认值 | 有效值 |
| ---------- | --------------------------------------------------------------------------------- | ---- | ------ | ------ | ------ |
| `label`    | 选项的标识符，显示在表单中。 支持 Markdown 用于粗体或斜体文本格式化和超文本链接。 | 必选 | 字符串 | -      | -      |
| `required` | 防止在元素完成之前提交表单。                                                      | 可选 | 布尔型 | false  | -      |
