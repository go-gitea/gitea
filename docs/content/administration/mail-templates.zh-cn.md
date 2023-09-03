---
date: "2023-05-23T09:00:00+08:00"
title: "邮件模板"
slug: "mail-templates"
sidebar_position: 45
toc: false
draft: false
aliases:
  - /zh-cn/mail-templates
menu:
  sidebar:
    parent: "administration"
    name: "邮件模板"
    sidebar_position: 45
    identifier: "mail-templates"
---

# 邮件模板

为了定制特定操作的电子邮件主题和内容，可以使用模板来自定义 Gitea。这些功能的模板位于 [`custom` 目录](administration/customizing-gitea.md) 下。
如果没有自定义的替代方案，Gitea 将使用内部模板作为默认模板。

自定义模板在 Gitea 启动时加载。对它们的更改在 Gitea 重新启动之前不会被识别。

## 支持模板的邮件通知

目前，以下通知事件使用模板：

| 操作名称   | 用途                                                                                    |
| ----------- | ------------------------------------------------------------------------------------------------------------ |
| `new`       | 创建了新的工单或合并请求。                                                                    |
| `comment`   | 在现有工单或合并请求中创建了新的评论。                                                          |
| `close`     | 关闭了工单或合并请求。                                                                         |
| `reopen`    | 重新打开了工单或合并请求。                                                                       |
| `review`    | 在合并请求中进行审查的首要评论。                                                               |
| `approve`   | 对合并请求进行批准的首要评论。                                                                 |
| `reject`    | 对合并请求提出更改请求的审查的首要评论。                                                       |
| `code`      | 关于合并请求的代码的单个评论。                                                                 |
| `assigned`  | 用户被分配到工单或合并请求。                                                                    |
| `default`   | 未包括在上述类别中的任何操作，或者当对应类别的模板不存在时使用的模板。                              |

特定消息类型的模板路径为：

```sh
custom/templates/mail/{操作类型}/{操作名称}.tmpl
```

其中 `{操作类型}` 是 `issue` 或 `pull`（针对合并请求），`{操作名称}` 是上述列出的操作名称之一。

例如，有关合并请求中的评论的电子邮件的特定模板是：

```sh
custom/templates/mail/pull/comment.tmpl
```

然而，并不需要为每个操作类型/名称组合创建模板。
使用回退系统来选择适当的模板。在此列表中，将使用 _第一个存在的_ 模板：

- 所需**操作类型**和**操作名称**的特定模板。
- 操作类型为 `issue` 和所需**操作名称**的模板。
- 所需**操作类型**和操作名称为 `default` 的模板。
- 操作类型为` issue` 和操作名称为 `default` 的模板。

唯一必需的模板是操作类型为 `issue` 操作名称为 `default` 的模板，除非用户在 `custom` 目录中覆盖了它。

## 模板语法

邮件模板是 UTF-8 编码的文本文件，需要遵循以下格式之一：

```
用于主题行的文本和宏
------------
用于邮件正文的文本和宏
```

或者

```
用于邮件正文的文本和宏
```

指定 _主题_ 部分是可选的（因此也是虚线分隔符）。在使用时，_主题_ 和 _邮件正文_ 模板之间的分隔符需要至少三个虚线；分隔符行中不允许使用其他字符。

_主题_ 和 _邮件正文_ 由 [Golang的模板引擎](https://golang.org/pkg/text/template/) 解析，并提供了为每个通知组装的 _元数据上下文_。上下文包含以下元素：

| 名称                 | 类型               | 可用性            | 用途                                                                                                                                                                                                                                             |
| -------------------- | ------------------ | ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `.FallbackSubject`   | string             | 始终可用        | 默认主题行。参见下文。                                                                                                                                                                                                                            |
| `.Subject`           | string             | 仅在正文中可用  | 解析后的 _主题_。                                                                                                                                                                                                                                 |
| `.Body`              | string             | 始终可用        | 工单、合并请求或评论的消息，从 Markdown 解析为 HTML 并进行了清理。请勿与 _邮件正文_ 混淆。                                                                                                                                                                 |
| `.Link`              | string             | 始终可用        | 源工单、合并请求或评论的地址。                                                                                                                                                                                                                    |
| `.Issue`             | models.Issue       | 始终可用        | 产生通知的工单（或合并请求）。要获取特定于合并请求的数据（例如 `HasMerged`），可以使用 `.Issue.PullRequest`，但需要注意，如果工单 _不是_ 合并请求，则该字段将为 `nil`。                                                                       |
| `.Comment`           | models.Comment     | 如果适用        | 如果通知是针对添加到工单或合并请求的评论，则其中包含有关评论的信息。                                                                                                                                                                             |
| `.IsPull`            | bool               | 始终可用        | 如果邮件通知与合并请求关联（即 `.Issue.PullRequest` 不为 `nil` ），则为 `true`。                                                                                                                                                                       |
| `.Repo`              | string         | 始终可用        | 仓库的名称，包括所有者名称（例如 `mike/stuff`）                                                                                                                                                                                                    |
| `.User`              | models.User        | 始终可用        | 事件来源仓库的所有者。要获取用户名（例如 `mike`），可以使用 `.User.Name`。                                                                                                                                                                           |
| `.Doer`              | models.User        | 始终可用        | 执行触发通知事件的操作的用户。要获取用户名（例如 `rhonda`），可以使用 `.Doer.Name`。                                                                                                                                                                |
| `.IsMention`         | bool               | 始终可用        | 如果此通知仅是因为在评论中提到了用户而生成的，并且收件人未订阅源，则为 `true`。如果收件人已订阅工单或仓库，则为 `false`。                                                                                                                            |
| `.SubjectPrefix`     | string             | 始终可用        | 如果通知是关于除工单或合并请求创建之外的其他内容，则为 `Re：`；否则为空字符串。                                                                                                                                                                      |
| `.ActionType`        | string             | 始终可用        | `"issue"` 或 `"pull"`。它将与实际的 _操作类型_ 对应，与选择的模板无关。                                                                                                                                                                                 |
| `.ActionName`        | string             | 始终可用        | 它将是上述操作类型之一（`new` ，`comment` 等），并与选择的模板对应。                                                                                                                                                                                 |
| `.ReviewComments`    | []models.Comment   | 始终可用        | 审查中的代码评论列表。评论文本将在 `.RenderedContent` 中，引用的代码将在 `.Patch` 中。                                                                                                                                                                |

所有名称区分大小写。

### 模板中的主题部分

用于邮件主题的模板引擎是 Golang 的 [`text/template`](https://golang.org/pkg/text/template/)。
有关语法的详细信息，请参阅链接的文档。

主题构建的步骤如下：

- 根据通知类型和可用的模板选择一个模板。
- 解析并解析模板（例如，将 `{{.Issue.Index}}` 转换为工单或合并请求的编号）。
- 将所有空格字符（例如 `TAB`，`LF` 等）转换为普通空格。
- 删除所有前导、尾随和多余的空格。
- 将字符串截断为前 256 个字母（字符）。

如果最终结果为空字符串，**或者**没有可用的主题模板（即所选模板不包含主题部分），将使用Gitea的**内部默认值**。

内部默认（回退）主题相当于：

```
{{.SubjectPrefix}}[{{.Repo}}] {{.Issue.Title}} (#{{.Issue.Index}})
```

例如：`Re: [mike/stuff] New color palette (#38)`

即使存在有效的主题模板，Gitea的默认主题也可以在模板的元数据中作为 `.FallbackSubject` 找到。

### 模板中的邮件正文部分

用于邮件正文的模板引擎是 Golang 的 [`html/template`](https://golang.org/pkg/html/template/)。
有关语法的详细信息，请参阅链接的文档。

邮件正文在邮件主题之后进行解析，因此还有一个额外的 _元数据_ 字段，即在考虑所有情况之后实际呈现的主题。

期望的结果是 HTML（包括结构元素，如`<html>`，`<body>`等）。可以通过 `<style>` 块、`class` 和 `style` 属性进行样式设置。但是，`html/template` 会进行一些 [自动转义](https://golang.org/pkg/html/template/#hdr-Contexts)，需要考虑这一点。

不支持附件（例如图像或外部样式表）。但是，也可以引用其他模板，例如以集中方式提供 `<style>` 元素的内容。外部模板必须放置在 `custom/mail` 下，并相对于该目录引用。例如，可以使用 `{{template styles/base}}` 包含 `custom/mail/styles/base.tmpl`。

邮件以 `Content-Type: multipart/alternative` 发送，因此正文以 HTML 和文本格式发送。通过剥离 HTML 标记来获取文本版本。

## 故障排除

邮件的呈现方式直接取决于邮件应用程序的功能。许多邮件客户端甚至不支持 HTML，因此显示生成邮件中包含的文本版本。

如果模板无法呈现，则只有在发送邮件时才会注意到。
如果主题模板失败，将使用默认主题，如果从 _邮件正文_ 中成功呈现了任何内容，则将使用该内容，忽略其他内容。

如果遇到问题，请检查 [Gitea的日志](administration/logging-config.md) 以获取错误消息。

## 示例

`custom/templates/mail/issue/default.tmpl`:

```html
[{{.Repo}}] @{{.Doer.Name}}
{{if eq .ActionName "new"}}
    创建了
{{else if eq .ActionName "comment"}}
    评论了
{{else if eq .ActionName "close"}}
    关闭了
{{else if eq .ActionName "reopen"}}
    重新打开了
{{else}}
    更新了
{{end}}
{{if eq .ActionType "issue"}}
    工单
{{else}}
    合并请求
{{end}}
#{{.Issue.Index}}: {{.Issue.Title}}
------------
<!DOCTYPE html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <title>{{.Subject}}</title>
</head>

<body>
    {{if .IsMention}}
    <p>
        您收到此邮件是因为 @{{.Doer.Name}} 提到了您。
    </p>
    {{end}}
    <p>
        <p>
        <a href="{{AppUrl}}/{{.Doer.LowerName}}">@{{.Doer.Name}}</a>
        {{if not (eq .Doer.FullName "")}}
            ({{.Doer.FullName}})
        {{end}}
        {{if eq .ActionName "new"}}
            创建了
        {{else if eq .ActionName "close"}}
            关闭了
        {{else if eq .ActionName "reopen"}}
            重新打开了
        {{else}}
            更新了
        {{end}}
        <a href="{{.Link}}">{{.Repo}}#{{.Issue.Index}}</a>。
        </p>
        {{if not (eq .Body "")}}
            <h3>消息内容：</h3>
            <hr>
            {{.Body | Str2html}}
        {{end}}
    </p>
    <hr>
    <p>
        <a href="{{.Link}}">在 Gitea 上查看</a>。
    </p>
</body>
</html>
```

该模板将生成以下内容：

### 主题

> [mike/stuff] @rhonda 在合并请求 #38 上进行了评论：New color palette

### 邮件正文

> [@rhonda](#)（Rhonda Myers）更新了 [mike/stuff#38](#)。
>
> #### 消息内容：
>
> \_********************************\_********************************
>
> Mike, I think we should tone down the blues a little.
>
> \_********************************\_********************************
>
> [在 Gitea 上查看](#)。

## 高级用法

模板系统包含一些函数，可用于进一步处理和格式化消息。以下是其中一些函数的列表：

| 函数名            | 参数        | 可用于       | 用法                                                                              |
| ----------------- | ----------- | ------------ | --------------------------------------------------------------------------------- |
| `AppUrl`          | -           | 任何地方     | Gitea 的 URL                                                                     |
| `AppName`         | -           | 任何地方     | 从 `app.ini` 中设置，通常为 "Gitea"                                               |
| `AppDomain`       | -           | 任何地方     | Gitea 的主机名                                                                   |
| `EllipsisString`  | string, int | 任何地方     | 将字符串截断为指定长度；根据需要添加省略号                                        |
| `Str2html`        | string      | 仅正文部分   | 通过删除其中的 HTML 标签对文本进行清理                                              |
| `Safe`            | string      | 仅正文部分   | 将输入作为 HTML 处理；可用于 `.ReviewComments.RenderedContent` 等字段               |

这些都是 _函数_，而不是元数据，因此必须按以下方式使用：

```html
像这样使用：         {{Str2html "Escape<my>text"}}
或者这样使用：       {{"Escape<my>text" | Str2html}}
或者这样使用：       {{AppUrl}}
但不要像这样使用：   {{.AppUrl}}
```
