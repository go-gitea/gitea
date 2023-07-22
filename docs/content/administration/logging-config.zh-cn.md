---
date: "2023-05-23T09:00:00+08:00"
title: "日志配置"
slug: "logging-config"
sidebar_position: 40
toc: false
draft: false
aliases:
  - /zh-cn/logging-configuration
menu:
  sidebar:
    parent: "administration"
    name: "日志配置"
    sidebar_position: 40
    identifier: "logging-config"
---

# 日志配置

Gitea 的日志配置主要由以下三种类型的组件组成：

- `[log]` 部分用于一般配置
- `[log.<mode-name>]` 部分用于配置不同的日志输出方式，也称为 "writer mode"，模式名称同时也作为 "writer name"
- `[log]` 部分还可以包含遵循 `logger.<logger-name>.<CONFIG-KEY>` 模式的子日志记录器的配置

默认情况下，已经有一个完全功能的日志输出，因此不需要重新定义。

## 收集日志以获取帮助

要收集日志以获取帮助和报告问题，请参阅 [需要帮助](help/support.md)。

## `[log]` 部分

在 Gitea 中，日志设施的配置在 `[log]` 部分及其子部分。

在顶层的 `[log]` 部分，可以放置以下配置项：

- `ROOT_PATH`：（默认值：**%(GITEA_WORK_DIR)/log**）：日志文件的基本路径。
- `MODE`：（默认值：**console**）：要用于默认日志记录器的日志输出列表。
- `LEVEL`：（默认值：**Info**）：要持久化的最严重的日志事件，不区分大小写。可能的值为：`Trace`、`Debug`、`Info`、`Warn`、`Error`、`Fatal`。
- `STACKTRACE_LEVEL`：（默认值：**None**）：对于此类及更严重的事件，将在记录时打印堆栈跟踪。

它还可以包含以下子日志记录器：

- `logger.router.MODE`：（默认值：**,**）：用于路由器日志记录器的日志输出列表。
- `logger.access.MODE`：（默认值：**_empty_**）：用于访问日志记录器的日志输出列表。默认情况下，访问日志记录器被禁用。
- `logger.xorm.MODE`：（默认值：**,**）：用于 XORM 日志记录器的日志输出列表。

将子日志记录器的模式设置为逗号（`,`）表示使用默认的全局 `MODE`。

## 快速示例

### 默认（空）配置

空配置等同于默认配置：

```ini
[log]
ROOT_PATH = %(GITEA_WORK_DIR)/log
MODE = console
LEVEL = Info
STACKTRACE_LEVEL = None
logger.router.MODE = ,
logger.xorm.MODE = ,
logger.access.MODE =

; 这是“控制台”模式的配置选项（由上面的 MODE=console 使用）
[log.console]
MODE = console
FLAGS = stdflags
PREFIX =
COLORIZE = true
```

这等同于将所有日志发送到控制台，并将默认的 Golang 日志也发送到控制台日志中。

这只是一个示例，默认情况下不需要将其写入配置文件中。

### 禁用路由日志并将一些访问日志记录到文件中

禁用路由日志，将访问日志（>=Warn）记录到 `access.log` 中：

```ini
[log]
logger.router.MODE =
logger.access.MODE = access-file

[log.access-file]
MODE = file
LEVEL = Warn
FILE_NAME = access.log
```

### 为不同的模式设置不同的日志级别

将默认日志（>=Warn）记录到 `gitea.log` 中，将错误日志记录到 `file-error.log` 中：

```ini
[log]
LEVEL = Warn
MODE = file, file-error

; 默认情况下，"file" 模式会将日志记录到 %(log.ROOT_PATH)/gitea.log，因此我们不需要设置它
; [log.file]

[log.file-error]
LEVEL = Error
FILE_NAME = file-error.log
```

## 日志输出（模式和写入器）

Gitea 提供以下日志写入器：

- `console` - 输出日志到 `stdout`（或 `stderr`，如果已在配置中设置）
- `file` - 输出日志到文件
- `conn` - 输出日志到套接字（网络或 Unix 套接字）

### 公共配置

某些配置适用于所有日志输出模式：

- `MODE` 是日志输出写入器的模式。它将默认为 ini 部分的模式名称。因此，`[log.console]` 将默认为 `MODE = console`。
- `LEVEL` 是此输出将记录的最低日志级别。
- `STACKTRACE_LEVEL` 是此输出将打印堆栈跟踪的最低日志级别。
- `COLORIZE` 对于 `console`，默认为 `true`，否则默认为 `false`。

#### `EXPRESSION`

`EXPRESSION` 表示日志事件必须匹配才能被输出写入器记录的正则表达式。
日志消息（去除颜色）或 `longfilename:linenumber:functionname` 必须匹配其中之一。
注意：整个消息或字符串不需要完全匹配。

请注意，此表达式将在写入器的 goroutine 中运行，而不是在日志事件的 goroutine 中运行。

#### `FLAGS`

`FLAGS` 表示在每条消息之前打印的前置日志上下文信息。
它是一个逗号分隔的字符串集。值的顺序无关紧要。

默认值为 `stdflags`（= `date,time,medfile,shortfuncname,levelinitial`）。

可能的值为：

- `none` 或 `,` - 无标志。
- `date` - 当地时区的日期：`2009/01/23`。
- `time` - 当地时区的时间：`01:23:23`。
- `microseconds` - 微秒精度：`01:23:23.123123`。假定有时间。
- `longfile` - 完整的文件名和行号：`/a/b/c/d.go:23`。
- `shortfile` - 文件名的最后一个部分和行号：`d.go:23`。
- `funcname` - 调用者的函数名：`runtime.Caller()`。
- `shortfuncname` - 函数名的最后一部分。覆盖 `funcname`。
- `utc` - 如果设置了日期或时间，则使用 UTC 而不是本地时区。
- `levelinitial` - 提供的级别的初始字符，放在方括号内，例如 `[I]` 表示 info。
- `level` - 在方括号内的级别，例如 `[INFO]`。
- `gopid` - 上下文的 Goroutine-PID。
- `medfile` - 文件名的最后 20 个字符 - 相当于 `shortfile,longfile`。
- `stdflags` - 相当于 `date,time,medfile,shortfuncname,levelinitial`。

### Console 模式

在此模式下，日志记录器将将日志消息转发到 Gitea 进程附加的 stdout 和 stderr 流。

对于 console 模式的日志记录器，如果不在 Windows 上，或者 Windows 终端可以设置为 ANSI 模式，或者是 cygwin 或 Msys 管道，则 `COLORIZE` 默认为 `true`。

设置：

- `STDERR`：**false**：日志记录器是否应将日志打印到 `stderr` 而不是 `stdout`。

### File 模式

在此模式下，日志记录器将将日志消息保存到文件中。

设置：

- `FILE_NAME`：要将日志事件写入的文件，相对于 `ROOT_PATH`，默认为 `%(ROOT_PATH)/gitea.log`。异常情况：访问日志默认为 `%(ROOT_PATH)/access.log`。
- `MAX_SIZE_SHIFT`：**28**：单个文件的最大大小位移。28 表示 256Mb。详细信息见下文。
- `LOG_ROTATE` **true**：是否轮转日志文件。
- `DAILY_ROTATE`：**true**：是否每天旋转日志。
- `MAX_DAYS`：**7**：在此天数之后删除旋转的日志文件。
- `COMPRESS`：**true**：默认情况下是否使用 gzip 压缩旧的日志文件。
- `COMPRESSION_LEVEL`：**-1**：压缩级别。详细信息见下文。

`MAX_SIZE_SHIFT` 通过将给定次数左移 1 (`1 << x`) 来定义文件的最大大小。
在 v1.17.3 版本时的确切行为可以在[这里](https://github.com/go-gitea/gitea/blob/v1.17.3/modules/setting/log.go#L185)中查看。

`COMPRESSION_LEVEL` 的有用值范围从 1 到（包括）9，其中较高的数字表示更好的压缩。
请注意，更好的压缩可能会带来更高的资源使用。
必须在前面加上 `-` 符号。

### Conn 模式

在此模式下，日志记录器将通过网络套接字发送日志消息。

设置：

- `ADDR`：**:7020**：设置要连接的地址。
- `PROTOCOL`：**tcp**：设置协议，可以是 "tcp"、"unix" 或 "udp"。
- `RECONNECT`：**false**：在连接丢失时尝试重新连接。
- `RECONNECT_ON_MSG`：**false**：为每条消息重新连接主机。

### "Router" 日志记录器

当 Gitea 的路由处理程序工作时，Router 日志记录器记录以下消息类型：

- `started` 消息将以 TRACE 级别记录
- `polling`/`completed` 路由将以 INFO 级别记录。异常情况："/assets" 静态资源请求也会以 TRACE 级别记录。
- `slow` 路由将以 WARN 级别记录
- `failed` 路由将以 WARN 级别记录

### "XORM" 日志记录器

为了使 XORM 输出 SQL 日志，还应将 `[database]` 部分中的 `LOG_SQL` 设置为 `true`。

### "Access" 日志记录器

"Access" 日志记录器是自 Gitea 1.9 版本以来的新日志记录器。它提供了符合 NCSA Common Log 标准的日志格式。虽然它具有高度可配置性，但在更改其模板时应谨慎。此日志记录器的主要好处是，Gitea 现在可以使用标准日志格式记录访问日志，因此可以使用标准工具进行分析。

您可以通过使用 `logger.access.MODE = ...` 来启用此日志记录器。

如果需要，可以通过更改 `ACCESS_LOG_TEMPLATE` 的值来更改 "Access" 日志记录器的格式。

请注意，访问日志记录器将以 `INFO` 级别记录，将此日志记录器的 `LEVEL` 设置为 `WARN` 或更高级别将导致不记录访问日志。

#### ACCESS_LOG_TEMPLATE

此值表示一个 Go 模板。其默认值为

```tmpl
{{.Ctx.RemoteHost}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.URL.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}" "{{.Ctx.Req.UserAgent}}"`
```

模板接收以下选项：

- `Ctx` 是 `context.Context`
- `Identity` 是 `SignedUserName`，如果用户未登录，则为 "-"
- `Start` 是请求的开始时间
- `ResponseWriter` 是 `http.ResponseWriter`

更改此模板时必须小心，因为它在标准的 panic 恢复陷阱之外运行。此模板应该尽可能简单，因为它会为每个请求运行一次。

## 释放和重新打开、暂停和恢复日志记录

如果您在 Unix 上运行，您可能希望释放和重新打开日志以使用 `logrotate` 或其他工具。
可以通过向运行中的进程发送 `SIGUSR1` 信号或运行 `gitea manager logging release-and-reopen` 命令来强制 Gitea 释放并重新打开其日志文件和连接。

或者，您可能希望暂停和恢复日志记录 - 可以通过使用 `gitea manager logging pause` 和 `gitea manager logging resume` 命令来实现。请注意，当日志记录暂停时，低于 INFO 级别的日志事件将不会存储，并且只会存储有限数量的事件。在暂停时，日志记录可能会阻塞，尽管是暂时性的，但会大大减慢 Gitea 的运行速度，因此建议仅暂停很短的时间。

### 在 Gitea 运行时添加和删除日志记录

可以使用 `gitea manager logging add` 和 `remove` 子命令在 Gitea 运行时添加和删除日志记录。
此功能只能调整正在运行的日志系统，不能用于启动未初始化的访问或路由日志记录器。如果您希望启动这些系统，建议调整 app.ini 并（优雅地）重新启动 Gitea 服务。

这些命令的主要目的是在运行中的系统上轻松添加临时日志记录器，以便调查问题，因为重新启动可能会导致问题消失。

## 使用 `logrotate` 而不是内置的日志轮转

Gitea 包含内置的日志轮转功能，对于大多数部署来说应该已经足够了。但是，如果您想使用 `logrotate` 工具：

- 在 `app.ini` 中将 `LOG_ROTATE` 设置为 `false`，禁用内置的日志轮转。
- 安装 `logrotate`。
- 根据部署要求配置 `logrotate`，有关配置语法细节，请参阅 `man 8 logrotate`。
  在 `postrotate/endscript` 块中通过 `kill -USR1` 或 `kill -10` 向 `gitea` 进程本身发送 `USR1` 信号，
  或者运行 `gitea manager logging release-and-reopen`（使用适当的环境设置）。
  确保配置适用于由 Gitea 日志记录器生成的所有文件，如上述部分所述。
- 始终使用 `logrotate /etc/logrotate.conf --debug` 来测试您的配置。
- 如果您正在使用 Docker 并从容器外部运行，您可以使用
  `docker exec -u $OS_USER $CONTAINER_NAME sh -c 'gitea manager logging release-and-reopen'`
  或 `docker exec $CONTAINER_NAME sh -c '/bin/s6-svc -1 /etc/s6/gitea/'`，或直接向 Gitea 进程本身发送 `USR1` 信号。

下一个 `logrotate` 作业将包括您的配置，因此不需要重新启动。
您还可以立即使用 `logrotate /etc/logrotate.conf --force` 重新加载 `logrotate`。
