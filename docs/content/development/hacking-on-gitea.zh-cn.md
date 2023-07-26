---
date: "2016-12-01T16:00:00+02:00"
title: "玩转 Gitea"
slug: "hacking-on-gitea"
sidebar_position: 10
toc: false
draft: false
aliases:
  - /zh-cn/hacking-on-gitea
menu:
  sidebar:
    parent: "development"
    name: "玩转 Gitea"
    sidebar_position: 10
    identifier: "hacking-on-gitea"
---

# Hacking on Gitea

## 快速入门

要获得快速工作的开发环境，您可以使用 Gitpod。

[![在 Gitpod 中打开](/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/go-gitea/gitea)

## 安装 Golang

您需要 [安装 go]( https://golang.org/doc/install ) 并设置您的 go 环境。

接下来，[使用 npm 安装 Node.js](https://nodejs.org/en/download/) ，这是构建
JavaScript 和 CSS 文件的必要工具。最低支持的 Node.js 版本是 @minNodeVersion@
并且推荐使用最新的 LTS 版本。

**注意** ：当执行需要外部工具的 make 任务时，比如
`make watch-backend`，Gitea 会自动下载并构建这些必要的组件。为了能够使用这些，你必须
将 `"$GOPATH"/bin` 目录加入到可执行路径上。如果你不把go bin目录添加到可执行路径你必须手动
指定可执行程序路径。

**注意2** ：Go版本 @minGoVersion@ 或更高版本是必须的。Gitea 使用 `gofmt` 来
格式化源代码。然而，`gofmt` 的结果可能因 `go` 的版本而有差异。因此推荐安装我们持续集成使用
的 Go版本。截至上次更新，Go 版本应该是 @goVersion@。

## 安装 Make

Gitea 大量使用 `Make` 来自动化任务和改进开发。本指南涵盖了如何安装 Make。

### 在 Linux 上

使用包管理器安装。

在 Ubuntu/Debian 上：

```bash
sudo apt-get install make
```

在 Fedora/RHEL/CentOS 上：

```bash
sudo yum install make
```

### 在 Windows 上

Make 的这三个发行版都可以在 Windows 上运行：

- [单个二进制构建]( http://www.equation.com/servlet/equation.cmd?fa=make )。复制到某处并添加到 `PATH`。
  - [32 位版本](http://www.equation.com/ftpdir/make/32/make.exe)
  - [64 位版本](http://www.equation.com/ftpdir/make/64/make.exe)
- [MinGW-w64](https://www.mingw-w64.org) / [MSYS2](https://www.msys2.org/)。
  - MSYS2 是一个工具和库的集合，为您提供一个易于使用的环境来构建、安装和运行本机 Windows 软件，它包括 MinGW-w64。
  - 在 MingGW-w64 中，二进制文件称为 `mingw32-make.exe` 而不是 `make.exe`。将 `bin` 文件夹添加到 `PATH`。
  - 在 MSYS2 中，您可以直接使用 `make`。请参阅 [MSYS2 移植](https://www.msys2.org/wiki/Porting/)。
  - 要使用 CGO_ENABLED（例如：SQLite3）编译 Gitea，您可能需要使用 [tdm-gcc](https://jmeubank.github.io/tdm-gcc/) 而不是 MSYS2 gcc，因为 MSYS2 gcc 标头缺少一些 Windows -只有 CRT 函数像 _beginthread 一样。
- [Chocolatey包管理器]( https://chocolatey.org/packages/make )。运行`choco install make`

**注意** ：如果您尝试在 Windows 命令提示符下使用 make 进行构建，您可能会遇到问题。建议使用上述提示（Git bash 或 MinGW），但是如果您只有命令提示符（或可能是 PowerShell），则可以使用 [set](https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/set_1) 命令，例如 `set TAGS=bindata`。

## 下载并克隆 Gitea 源代码

获取源代码的推荐方法是使用 `git clone`。

```bash
git clone https://github.com/go-gitea/gitea
```

（自从go modules出现后，不再需要构建 go 项目从 `$GOPATH` 中获取，因此不再推荐使用 `go get` 方法。）

## 派生 Gitea

如上所述下载主要的 Gitea 源代码。然后，派生 [Gitea 仓库](https://github.com/go-gitea/gitea)，
并为您的本地仓库切换 git 远程源，或添加另一个远程源：

```bash
# 将原来的 Gitea origin 重命名为 upstream
git remote rename origin upstream
git remote add origin "git@github.com:$GITHUB_USERNAME/gitea.git"
git fetch --all --prune
```

或者：

```bash
# 为我们的 fork 添加新的远程
git remote add "$FORK_NAME" "git@github.com:$GITHUB_USERNAME/gitea.git"
git fetch --all --prune
```

为了能够创建合并请求，应将分叉存储库添加为 Gitea 本地仓库的远程，否则无法推送更改。

## 构建 Gitea（基本）

看看我们的
[说明](installation/from-source.md)
关于如何[从源代码构建](installation/from-source.md) 。

从源代码构建的最简单推荐方法是：

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build
```

`build` 目标将同时执行 `frontend` 和 `backend` 子目标。如果存在 `bindata` 标签，资源文件将被编译成二进制文件。建议在进行前端开发时省略 `bindata` 标签，以便实时反映更改。

有关所有可用的 `make` 目标，请参阅 `make help`。另请参阅 [`.drone.yml`](https://github.com/go-gitea/gitea/blob/main/.drone.yml) 以了解我们的持续集成是如何工作的。

## 持续构建

要在源文件更改时运行并持续构建：

```bash
# 对于前端和后端
make watch

# 或者：只看前端文件（html/js/css）
make watch-frontend

# 或者：只看后端文件 (go)
make watch-backend
```

在 macOS 上，监视所有后端源文件可能会达到默认的打开文件限制，这可以通过当前 shell 的 `ulimit -n 12288` 或所有未来 shell 的 shell 启动文件来增加。

### 格式化、代码分析和拼写检查

我们的持续集成将拒绝未通过代码检查（包括格式检查、代码分析和拼写检查）的 PR。

你应该格式化你的代码：

```bash
make fmt
```

并检查源代码：

```bash
# lint 前端和后端代码
make lint
# 仅 lint 后端代码
make lint-backend
```

**注意** ：`gofmt` 的结果取决于 `go` 的版本。您应该运行与持续集成相同的 go 版本。

### 处理 JS 和 CSS

前端开发应遵循 [Guidelines for Frontend Development](contributing/guidelines-frontend.md)。

要使用前端资源构建，请使用上面提到的“watch-frontend”目标或只构建一次：

```bash
make build && ./gitea
```

在提交之前，确保 linters 通过：

```bash
make lint-frontend
```

### 配置本地 ElasticSearch 实例

使用 docker 启动本地 ElasticSearch 实例：

```sh
mkdir -p $(pwd) /data/elasticsearch
sudo chown -R 1000:1000 $(pwd) /data/elasticsearch
docker run --rm --memory= "4g" -p 127.0.0.1:9200:9200 -p 127.0.0.1:9300:9300 -e "discovery.type=single-node" -v "$(pwd)/data /elasticsearch:/usr/share/elasticsearch/data" docker.elastic.co/elasticsearch/elasticsearch:7.16.3
```

配置`app.ini`：

```ini
[indexer]
ISSUE_INDEXER_TYPE = elasticsearch
ISSUE_INDEXER_CONN_STR = http://elastic:changeme@localhost:9200
REPO_INDEXER_ENABLED = true
REPO_INDEXER_TYPE = elasticsearch
REPO_INDEXER_CONN_STR = http://elastic:changeme@localhost:9200
```

### 构建和添加 SVGs

SVG 图标是使用 `make svg` 目标构建的，该目标将 `build/generate-svg.js` 中定义的图标源编译到输出目录 `public/img/svg` 中。可以在 `web_src/svg` 目录中添加自定义图标。

### 构建 Logo

Gitea Logo的 PNG 和 SVG 版本是使用 `TAGS="gitea" make generate-images` 目标从单个 SVG 源文件 assets/logo.svg 构建的。要运行它，Node.js 和 npm 必须可用。

通过更新 `assets/logo.svg` 并运行 `make generate-images`，同样的过程也可用于从 SVG 源文件生成自定义 Logo PNG。忽略 gitea 编译选项将仅更新用户指定的 LOGO 文件。

### 更新 API

创建新的 API 路由或修改现有的 API 路由时，您**必须**
更新和/或创建 [Swagger](https://swagger.io/docs/specification/2-0/what-is-swagger/)
这些使用 [go-swagger](https://goswagger.io/) 评论的文档。
[规范]( https://goswagger.io/use/spec.html#annotation-syntax )中描述了这些注释的结构。
如果您想了解更多有关 Swagger 结构的信息，可以查看
[Swagger 2.0 文档](https://swagger.io/docs/specification/2-0/basic-structure/)
或与添加新 API 端点的先前 PR 进行比较，例如 [PR #5483](https://github.com/go-gitea/gitea/pull/5843/files#diff-2e0a7b644cf31e1c8ef7d76b444fe3aaR20)

您应该注意不要破坏下游用户依赖的 API。在稳定的 API 上，一般来说添加是可以接受的，但删除
或对 API 进行根本性更改将会被拒绝。

创建或更改 API 端点后，请用以下命令重新生成 Swagger 文档：

```bash
make generate-swagger
```

您应该验证生成的 Swagger 文件并使用以下命令对其进行拼写检查：

```bash
make swagger-validate misspell-check
```

您应该提交更改后的 swagger JSON 文件。持续集成服务器将使用以下方法检查是否已完成：

```bash
make swagger-check
```

**注意** ：请注意，您应该使用 Swagger 2.0 文档，而不是 OpenAPI 3 文档。

### 创建新的配置选项

创建新的配置选项时，将它们添加到 `modules/setting` 的对应文件。您应该将信息添加到 `custom/conf/app.ini`
并到[配置备忘单](administration/config-cheat-sheet.md)
在 `docs/content/doc/advanced/config-cheat-sheet.zh-cn.md` 中找到

### 更改Logo

更改 Gitea Logo SVG 时，您将需要运行并提交结果的：

```bash
make generate-images
```

这将创建必要的 Gitea 图标和其他图标。

### 数据库迁移

如果您对数据库中的任何数据库持久结构进行重大更改
`models/` 目录，您将需要进行新的迁移。可以找到这些
在 `models/migrations/` 中。您可以确保您的迁移适用于主要
数据库类型使用：

```bash
make test-sqlite-migration # 将 SQLite 切换为适当的数据库
```

## 测试

Gitea 运行两种类型的测试：单元测试和集成测试。

### 单元测试

`go test` 系统中的`*_test.go` 涵盖了单元测试。
您可以设置环境变量 `GITEA_UNIT_TESTS_LOG_SQL=1` 以在详细模式下运行测试时显示所有 SQL 语句（即设置`GOTESTFLAGS=-v` 时）。

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make test # Runs the unit tests
```

### 集成测试

单元测试不会也不能完全单独测试 Gitea。因此，我们编写了集成测试；但是，这些依赖于数据库。

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build test-sqlite
```

将在 SQLite 环境中运行集成测试。集成测试需要安装 `git lfs`。其他数据库测试可用，但
可能需要适应当地环境。

看看 [`tests/integration/README.md`](https://github.com/go-gitea/gitea/blob/main/tests/integration/README.md) 有关更多信息以及如何运行单个测试。

### 测试 PR

我们的持续集成将测试代码是否通过了单元测试，并且所有支持的数据库都将在 Docker 环境中通过集成测试。
还将测试从几个最新版本的 Gitea 迁移。

请在PR中附带提交适当的单元测试和集成测试。

## 网站文档

该网站的文档位于 `docs/` 中。如果你改变了文档内容，你可以使用以下测试方法进行持续集成：

```bash
# 来自 Gitea 中的 docs 目录
make trans-copy clean build
```

运行此任务依赖于 [Hugo](https://gohugo.io/)。请注意：这可能会生成一些未跟踪的 Git 对象，
需要被清理干净。

## Visual Studio Code

`contrib/ide/vscode` 中为 Visual Studio Code 提供了 `launch.json` 和 `tasks.json`。查看
[`contrib/ide/README.md`](https://github.com/go-gitea/gitea/blob/main/contrib/ide/README.md) 了解更多信息。

## Goland

单击 `/main.go` 中函数 `func main()` 上的 `Run Application` 箭头
可以快速启动一个可调试的 Gitea 实例。

`Run/Debug Configuration` 中的 `Output Directory` 必须设置为
gitea 项目目录（包含 `main.go` 和 `go.mod`），
否则，启动实例的工作目录是 GoLand 的临时目录
并防止 Gitea 在开发环境中加载动态资源（例如：模板）。

要在 GoLand 中使用 SQLite 运行单元测试，请设置 `-tags sqlite,sqlite_unlock_notify`
在 `运行/调试配置` 的 `Go 工具参数` 中。

## 提交 PR

对更改感到满意后，将它们推送并打开拉取请求。它建议您允许 Gitea Managers 和 Owners 修改您的 PR
分支，因为我们需要在合并之前将其更新为 main 和/或可能是能够直接帮助解决问题。

任何 PR 都需要 Gitea 维护者的两次批准，并且需要通过持续集成。看看我们的
[CONTRIBUTING.md](https://github.com/go-gitea/gitea/blob/main/CONTRIBUTING.md)
文档。

如果您需要更多帮助，请访问 [Discord](https://discord.gg/gitea) #Develop 频道
并在那里聊天。

现在，您已准备好 Hacking Gitea。
