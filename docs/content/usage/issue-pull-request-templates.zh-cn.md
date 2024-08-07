---
date: "2022-09-07T16:00:00+08:00"
title: "工单与合并请求模板"
slug: "issue-pull-request-templates"
sidebar_position: 15
toc: true
draft: false
aliases:
  - /zh-cn/issue-pull-request-templates
menu:
  sidebar:
    parent: "usage"
    name: "工单与合并请求模板"
    sidebar_position: 15
    identifier: "issue-pull-request-templates"
---

# 从模板创建工单与合并请求

开发者可以利用问题模板创建工单与合并请求，其目的在于规范参与者的语言表达。

## 模板介绍

Gitea 支持两种格式的模板：Markdown 和 YAML。

### Markdown 模板

在 Gitea 中存在两种用途的 Markdown 模板：

- `ISSUE_TEMPLATE/bug-report.md` 用于规范工单的 Markdown 文本描述
- `PULL_REQUEST_TEMPLATE.md` 用于规范合并请求的 Markdown 文本描述

对于以上 Markdown 模板，我们推荐您将它们放置到项目目录 `.gitea` 进行收纳。

### YAML 模板

用 YAML 语法编写的模板相比 Markdown 可以实现更丰富的功能，利用表单实现诸如：问卷调查、字符校验。在 Gitea 中的 YAML 同样支持两种用途：

- `ISSUE_TEMPLATE/bug-report.yaml` 用于创建问卷调查形式的工单
- `PULL_REQUEST_TEMPLATE.yaml` 用于创建表单形式的合并请求

对于以上 YAML 模板，我们同样推荐您将它们放置到项目目录 `.gitea` 进行收纳。

##### 表单支持通过 URL 查询参数传值

当新建工单页面 URL 以 `?title=Issue+Title&body=Issue+Text` 为查询参数，表单将使用其中的参数（key-value）填充表单内容。

### Gitea 支持的模板文件路径

工单模板文件名:

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
- `.gitea/issue_template.yml`
- `.github/ISSUE_TEMPLATE.md`
- `.github/ISSUE_TEMPLATE.yaml`
- `.github/ISSUE_TEMPLATE.yml`
- `.github/issue_template.md`
- `.github/issue_template.yaml`
- `.github/issue_template.yml`

合并请求模板:

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

#### 工单模板目录

由于工单存在多种类型，Gitea 支持将工单模板统一收纳到 `ISSUE_TEMPLATE` 目录。以下是 Gitea 支持的工单模板目录:

- `ISSUE_TEMPLATE`
- `issue_template`
- `.gitea/ISSUE_TEMPLATE`
- `.gitea/issue_template`
- `.github/ISSUE_TEMPLATE`
- `.github/issue_template`
- `.gitlab/ISSUE_TEMPLATE`
- `.gitlab/issue_template`

目录支持混合存放 Markdown (`.md`) 或 YAML (`.yaml`/`.yml`) 格式的工单模板。另外，合并请求模板不支持目录存放。

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

上面的示例表示用户从列表中选择一个工单模板时，列表会展示模板名称 `Template Name` 和模板描述 `This template is for testing!`。 同时，标题会预先填充为 `[TEST]`，而正文将预先填充 `This is the template!`。 最后，Issue 还会被分配两个标签，`bug` 和 `help needed`，并且将问题指向 `main` 分支。

## YAML 模板语法

YAML 模板格式如下，相比 Markdown 模板提供了更多实用性的功能。

```yaml
name: 表单名称
about: 表单描述
title: 默认标题
body: 主体内容
  type: 定义表单元素类型
    id: 定义表单标号
    attributes: 扩展的属性
    validations: 内容校验
```

下例 YAML 配置文件完整定义了一个用于提交 bug 的问卷调查。

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

### Markdown 段落

您可以在 YAML 模板中使用 `markdown` 元素为开发者提供额外的上下文支撑，这部分内容会作为创建工单的提示但不会作为工单内容提交。

`attributes` 子项提供了以下扩展能力：

| 键      | 描述                           | 必选 | 类型   | 默认值 | 有效值 |
| ------- | ------------------------------ | ---- | ------ | ------ | ------ |
| `value` | 渲染的文本。支持 Markdown 格式 | 必选 | 字符串 | -      | -      |

### Textarea 多行文本输入框

您可以使用 `textarea` 元素在表单中添加多行文本输入框。 除了输入文本，开发者还可以在 `textarea` 区域附加文件。

`attributes` 子项提供了以下扩展能力：

| 键            | 描述                                                                                                  | 必选 | 类型   | 默认值   | 有效值             |
| ------------- | ----------------------------------------------------------------------------------------------------- | ---- | ------ | -------- | ------------------ |
| `label`       | 预期用户输入的简短描述，也以表单形式显示。                                                            | 必选 | 字符串 | -        | -                  |
| `description` | 提供上下文或指导的文本区域的描述，以表单形式显示。                                                    | 可选 | 字符串 | 空字符串 | -                  |
| `placeholder` | 半透明的占位符，在文本区域空白时呈现                                                                  | 可选 | 字符串 | 空字符串 | -                  |
| `value`       | 在文本区域中预填充的文本。                                                                            | 可选 | 字符串 | -        | -                  |
| `render`      | 如果提供了值，提交的文本将格式化为代码块。 提供此键时，文本区域将不会扩展到文件附件或 Markdown 编辑。 | 可选 | 字符串 | -        | Gitea 支持的语言。 |

`validations` 子项提供以下文本校验参数：

| 键         | 描述                         | 必选 | 类型   | 默认值 | 有效值 |
| ---------- | ---------------------------- | ---- | ------ | ------ | ------ |
| `required` | 防止在元素完成之前提交表单。 | 可选 | 布尔型 | false  | -      |

### Input 单行输入框

您可以使用 `input` 元素添加单行文本字段到表单。

`attributes` 子项提供了以下扩展能力：

| 键            | 描述                                           | 必选 | 类型   | 默认值   | 有效值 |
| ------------- | ---------------------------------------------- | ---- | ------ | -------- | ------ |
| `label`       | 预期用户输入的简短描述，也以表单形式显示。     | 必选 | 字符串 | -        | -      |
| `description` | 提供上下文或指导的字段的描述，以表单形式显示。 | 可选 | 字符串 | 空字符串 | -      |
| `placeholder` | 半透明的占位符，在字段空白时呈现。             | 可选 | 字符串 | 空字符串 | -      |
| `value`       | 字段中预填的文本。                             | 可选 | 字符串 | -        | -      |

`validations` 子项提供以下文本校验参数：

| 键          | 描述                             | 必选 | 类型   | 默认值 | 有效值                                                         |
| ----------- | -------------------------------- | ---- | ------ | ------ | -------------------------------------------------------------- |
| `required`  | 防止在未填内容时提交表单。       | 可选 | 布尔型 | false  | -                                                              |
| `is_number` | 防止在未填数字时提交表单。       | 可选 | 布尔型 | false  | -                                                              |
| `regex`     | 直到满足了与正则表达式匹配的值。 | 可选 | 字符串 | -      | [正则表达式](https://en.wikipedia.org/wiki/Regular_expression) |

### Dropdown 下拉菜单

您可以使用 `dropdown` 元素在表单中添加下拉菜单。

`attributes` 子项提供了以下扩展能力：

| 键            | 描述                                                      | 必选 | 类型       | 默认值   | 有效值 |
| ------------- | --------------------------------------------------------- | ---- | ---------- | -------- | ------ |
| `label`       | 预期用户输入的简短描述，以表单形式显示。                  | 必选 | 字符串     | -        | -      |
| `description` | 提供上下文或指导的下拉列表的描述，以表单形式显示。        | 可选 | 字符串     | 空字符串 | -      |
| `multiple`    | 确定用户是否可以选择多个选项。                            | 可选 | 布尔型     | false    | -      |
| `options`     | 用户可以选择的选项列表。 不能为空，所有选择必须是不同的。 | 必选 | 字符串数组 | -        | -      |

`validations` 子项提供以下文本校验参数：

| 键         | 描述                         | 必选 | 类型   | 默认值 | 有效值 |
| ---------- | ---------------------------- | ---- | ------ | ------ | ------ |
| `required` | 防止在元素完成之前提交表单。 | 可选 | 布尔型 | false  | -      |

### Checkboxes 复选框

您可以使用 `checkboxes` 元素添加一组复选框到表单。

`attributes` 子项提供了以下扩展能力：

| 键            | 描述                                                  | 必选 | 类型   | 默认值   | 有效值 |
| ------------- | ----------------------------------------------------- | ---- | ------ | -------- | ------ |
| `label`       | 预期用户输入的简短描述，以表单形式显示。              | 必选 | 字符串 | -        | -      |
| `description` | 复选框集的描述，以表单形式显示。 支持 Markdown 格式。 | 可选 | 字符串 | 空字符串 | -      |
| `options`     | 用户可以选择的复选框列表。 有关语法，请参阅下文。     | 必选 | 数组   | -        | -      |

对于 `options`，您可以设置以下参数：

| 键         | 描述                                                                              | 必选 | 类型   | 默认值 | 有效值 |
| ---------- | --------------------------------------------------------------------------------- | ---- | ------ | ------ | ------ |
| `label`    | 选项的标识符，显示在表单中。 支持 Markdown 用于粗体或斜体文本格式化和超文本链接。 | 必选 | 字符串 | -      | -      |
| `required` | 防止在元素完成之前提交表单。                                                      | 可选 | 布尔型 | false  | -      |
