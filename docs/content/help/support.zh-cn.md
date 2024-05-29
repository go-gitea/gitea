---
date: "2017-01-20T15:00:00+08:00"
title: "需要帮助"
slug: "support"
sidebar_position: 20
toc: false
draft: false
aliases:
  - /zh-cn/seek-help
menu:
  sidebar:
    parent: "help"
    name: "需要帮助"
    sidebar_position: 20
    identifier: "support"
---

# 支持选项

- [付费商业支持](https://about.gitea.com/)
- [Discord](https://discord.gg/Gitea)
- [论坛](https://forum.gitea.com/)
- [Matrix](https://matrix.to/#/#gitea-space:matrix.org)
  - 注意：大多数 Matrix 频道都与 Discord 中的对应频道桥接，可能在桥接过程中会出现一定程度的不稳定性。
- 中文支持
  - [Discourse 中文分类](https://forum.gitea.com/c/5-category/5)
  - QQ 群 328432459

# Bug 报告

如果您发现了 Bug，请在 GitHub 上 [创建一个问题](https://github.com/go-gitea/gitea/issues)。

**注意：** 在请求支持时，可能需要准备以下信息，以便帮助者获得所需的所有信息：

1. 您的 `app.ini`（将任何敏感数据进行必要的清除）。
2. 您看到的任何错误消息。
3. Gitea 日志以及与情况相关的所有其他日志。
   - 收集 `trace` / `debug` 级别的日志更有用（参见下一节）。
   - 在使用 systemd 时，使用 `journalctl --lines 1000 --unit gitea` 收集日志。
   - 在使用 Docker 时，使用 `docker logs --tail 1000 <gitea-container>` 收集日志。
4. 可重现的步骤，以便他人能够更快速、更容易地重现和理解问题。
   - [demo.gitea.com](https://demo.gitea.com) 可用于重现问题。
5. 如果遇到慢速/挂起/死锁等问题，请在出现问题时报告堆栈跟踪。
   转到 "Site Admin" -> "Monitoring" -> "Stacktrace" -> "Download diagnosis report"。

# 高级 Bug 报告提示

## 更多日志的配置选项

默认情况下，日志以 `info` 级别输出到控制台。
如果您需要设置日志级别和/或从文件中收集日志，
您只需将以下配置复制到您的 `app.ini` 中（删除所有其他 `[log]` 部分），
然后您将在 Gitea 的日志目录中找到 `*.log` 文件（默认为 `%(GITEA_WORK_DIR)/log`）。

```ini
; 要显示所有 SQL 日志，您还可以在 [database] 部分中设置 LOG_SQL=true
[log]
LEVEL=debug
MODE=console,file
```

## 使用命令行收集堆栈跟踪

Gitea 可以使用 Golang 的 pprof 处理程序和工具链来收集堆栈跟踪和其他运行时信息。

如果 Web UI 停止工作，您可以尝试通过命令行收集堆栈跟踪：

1. 设置 app.ini：

    ```
    [server]
    ENABLE_PPROF = true
    ```

2. 重新启动 Gitea

3. 尝试触发bug，当请求卡住一段时间，使用或浏览器访问：获取堆栈跟踪。
`curl http://127.0.0.1:6060/debug/pprof/goroutine?debug=1`
