---
date: "2023-05-23T09:00:00+08:00"
title: "外部渲染器"
slug: "external-renderers"
sidebar_position: 60
toc: false
draft: false
aliases:
  - /zh-cn/external-renderers
menu:
  sidebar:
    parent: "administration"
    name: "外部渲染器"
    sidebar_position: 60
    identifier: "external-renderers"
---

# 自定义文件渲染配置

Gitea 通过外部二进制文件支持自定义文件渲染（例如 Jupyter notebooks、asciidoc 等），只需要进行以下步骤：

- 安装外部二进制文件
- 在您的 `app.ini` 文件中添加一些配置
- 重新启动 Gitea 实例

此功能支持整个文件的渲染。如果您想要在 Markdown 中渲染代码块，您需要使用 JavaScript 进行一些操作。请参阅 [自定义 Gitea 配置](administration/customizing-gitea.md) 页面上的一些示例。

## 安装外部二进制文件

为了通过外部二进制文件进行文件渲染，必须安装它们的关联软件包。
如果您正在使用 Docker 镜像，则您的 `Dockerfile` 应该包含以下内容：

```docker
FROM gitea/gitea:@version@
[...]

COPY custom/app.ini /data/gitea/conf/app.ini
[...]

RUN apk --no-cache add asciidoctor freetype freetype-dev gcc g++ libpng libffi-dev pandoc python3-dev py3-pyzmq pipx
# 安装其他您需要的外部渲染器的软件包

RUN pipx install jupyter docutils --include-deps
# 在上面添加您需要安装的任何其他 Python 软件包
```

## `app.ini` 文件配置

在您的自定义 `app.ini` 文件中为每个外部渲染器添加一个 `[markup.XXXXX]` 部分：

```ini
[markup.asciidoc]
ENABLED = true
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoctor -s -a showtitle --out-file=- -"
; 输入不是标准输入而是文件
IS_INPUT_FILE = false

[markup.jupyter]
ENABLED = true
FILE_EXTENSIONS = .ipynb
RENDER_COMMAND = "jupyter nbconvert --stdin --stdout --to html --template basic"
IS_INPUT_FILE = false

[markup.restructuredtext]
ENABLED = true
FILE_EXTENSIONS = .rst
RENDER_COMMAND = "timeout 30s pandoc +RTS -M512M -RTS -f rst"
IS_INPUT_FILE = false
```

如果您的外部标记语言依赖于在生成的 HTML 元素上的额外类和属性，您可能需要启用自定义的清理策略。Gitea 使用 [`bluemonday`](https://godoc.org/github.com/microcosm-cc/bluemonday) 包作为我们的 HTML 清理器。下面的示例可以用于支持从 [`pandoc`](https://pandoc.org/) 输出的服务器端 [KaTeX](https://katex.org/) 渲染结果。

```ini
[markup.sanitizer.TeX]
; Pandoc 渲染 TeX 段落为带有 "math" 类的 <span> 元素，根据上下文可能还带有 "inline" 或 "display" 类。
; - 请注意，这与我们的 Markdown 解析器中内置的数学支持不同，后者使用 <code> 元素。
ELEMENT = span
ALLOW_ATTR = class
REGEXP = ^\s*((math(\s+|$)|inline(\s+|$)|display(\s+|$)))+

[markup.markdown]
ENABLED         = true
FILE_EXTENSIONS = .md,.markdown
RENDER_COMMAND  = pandoc -f markdown -t html --katex
```

您必须在每个部分中定义 `ELEMENT` 和 `ALLOW_ATTR`。

要定义多个条目，请添加唯一的字母数字后缀（例如，`[markup.sanitizer.1]` 和 `[markup.sanitizer.something]`）。

要仅为特定的外部渲染器应用清理规则，它们必须使用渲染器名称，例如 `[markup.sanitizer.asciidoc.rule-1]`、`[markup.sanitizer.<renderer>.rule-1]`。

**注意**：如果规则在渲染器 ini 部分之前定义，或者名称与渲染器不匹配，它将应用于所有渲染器。

完成配置更改后，请重新启动 Gitea 以使更改生效。

**注意**：在 Gitea 1.12 之前，存在一个名为 `markup.sanitiser` 的单个部分，其中的键被重新定义为多个规则，但是，这种配置方法存在重大问题，需要通过多个部分进行配置。

### 示例：HTML

直接渲染 HTML 文件：

```ini
[markup.html]
ENABLED         = true
FILE_EXTENSIONS = .html,.htm
RENDER_COMMAND  = cat
; 输入不是标准输入，而是文件
IS_INPUT_FILE   = true

[markup.sanitizer.html.1]
ELEMENT = div
ALLOW_ATTR = class

[markup.sanitizer.html.2]
ELEMENT = a
ALLOW_ATTR = class
```

请注意：此示例中的配置将允许渲染 HTML 文件，并使用 `cat` 命令将文件内容输出为 HTML。此外，配置中的两个清理规则将允许 `<div>` 和 `<a>` 元素使用 `class` 属性。

在进行配置更改后，请重新启动 Gitea 以使更改生效。

### 示例：Office DOCX

使用 [`pandoc`](https://pandoc.org/) 显示 Office DOCX 文件：

```ini
[markup.docx]
ENABLED = true
FILE_EXTENSIONS = .docx
RENDER_COMMAND = "pandoc --from docx --to html --self-contained --template /path/to/basic.html"

[markup.sanitizer.docx.img]
ALLOW_DATA_URI_IMAGES = true
```

在此示例中，配置将允许显示 Office DOCX 文件，并使用 `pandoc` 命令将文件转换为 HTML 格式。同时，清理规则中的 `ALLOW_DATA_URI_IMAGES` 设置为 `true`，允许使用 Data URI 格式的图片。

模板文件的内容如下：

```
$body$
```

### 示例：Jupyter Notebook

使用 [`nbconvert`](https://github.com/jupyter/nbconvert) 显示 Jupyter Notebook 文件：

```ini
[markup.jupyter]
ENABLED = true
FILE_EXTENSIONS = .ipynb
RENDER_COMMAND = "jupyter-nbconvert --stdin --stdout --to html --template basic"

[markup.sanitizer.jupyter.img]
ALLOW_DATA_URI_IMAGES = true
```

在此示例中，配置将允许显示 Jupyter Notebook 文件，并使用 `nbconvert` 命令将文件转换为 HTML 格式。同样，清理规则中的 `ALLOW_DATA_URI_IMAGES` 设置为 `true`，允许使用 Data URI 格式的图片。

在进行配置更改后，请重新启动 Gitea 以使更改生效。

## 自定义 CSS

在 `.ini` 文件中，可以使用 `[markup.XXXXX]` 的格式指定外部渲染器，并且由外部渲染器生成的 HTML 将被包装在一个带有 `markup` 和 `XXXXX` 类的 `<div>` 中。`markup` 类提供了预定义的样式（如果 `XXXXX` 是 `markdown`，则使用 `markdown` 类）。否则，您可以使用这些类来针对渲染的 HTML 内容进行定制样式。

因此，您可以编写一些 CSS 样式：

```css
.markup.XXXXX html {
  font-size: 100%;
  overflow-y: scroll;
  -webkit-text-size-adjust: 100%;
  -ms-text-size-adjust: 100%;
}

.markup.XXXXX body {
  color: #444;
  font-family: Georgia, Palatino, 'Palatino Linotype', Times, 'Times New Roman', serif;
  font-size: 12px;
  line-height: 1.7;
  padding: 1em;
  margin: auto;
  max-width: 42em;
  background: #fefefe;
}

.markup.XXXXX p {
  color: orangered;
}
```

将您的样式表添加到自定义目录中，例如 `custom/public/assets/css/my-style-XXXXX.css`，并使用自定义的头文件 `custom/templates/custom/header.tmpl` 进行导入：

```html
<link rel="stylesheet" href="{{AppSubUrl}}/assets/css/my-style-XXXXX.css" />
```

通过以上步骤，您可以将自定义的 CSS 样式应用到特定的外部渲染器，使其具有所需的样式效果。
