---
date: "2018-05-21T15:00:00+00:00"
title: "Support Options"
slug: "support"
sidebar_position: 20
toc: false
draft: false
aliases:
  - /en-us/seek-help
menu:
  sidebar:
    parent: "help"
    name: "Support Options"
    sidebar_position: 20
    identifier: "support"
---

# Support Options

- [Paid Commercial Support](https://about.gitea.com/)
- [Discord](https://discord.gg/Gitea)
- [Discourse Forum](https://discourse.gitea.io/)
- [Matrix](https://matrix.to/#/#gitea-space:matrix.org)
  - NOTE: Most of the Matrix channels are bridged with their counterpart in Discord and may experience some degree of flakiness with the bridge process.
- Chinese Support
  - [Discourse Chinese Category](https://discourse.gitea.io/c/5-category/5)
  - QQ Group 328432459

# Bug Report

If you found a bug, please [create an issue on GitHub](https://github.com/go-gitea/gitea/issues).

**NOTE:** When asking for support, it may be a good idea to have the following available so that the person helping has all the info they need:

1. Your `app.ini` (with any sensitive data scrubbed as necessary).
2. Any error messages you are seeing.
3. The Gitea logs, and all other related logs for the situation.
   - It's more useful to collect `trace` / `debug` level logs (see the next section).
   - When using systemd, use `journalctl --lines 1000 --unit gitea` to collect logs.
   - When using docker, use `docker logs --tail 1000 <gitea-container>` to collect logs.
4. Reproducible steps so that others could reproduce and understand the problem more quickly and easily.
   - [try.gitea.io](https://try.gitea.io) could be used to reproduce the problem.
5. If you encounter slow/hanging/deadlock problems, please report the stacktrace when the problem occurs.
   Go to the "Site Admin" -> "Monitoring" -> "Stacktrace" -> "Download diagnosis report".

# Advanced Bug Report Tips

## More Config Options for Logs

By default, the logs are outputted to console with `info` level.
If you need to set log level and/or collect logs from files,
you could just copy the following config into your `app.ini` (remove all other `[log]` sections),
then you will find the `*.log` files in Gitea's log directory (default: `%(GITEA_WORK_DIR)/log`).

```ini
; To show all SQL logs, you can also set LOG_SQL=true in the [database] section
[log]
LEVEL=debug
MODE=console,file
```

## Collecting Stacktrace by Command Line

Gitea could use Golang's pprof handler and toolchain to collect stacktrace and other runtime information.

If the web UI stops working, you could try to collect the stacktrace by command line:

1. Set `app.ini`:

    ```
    [server]
    ENABLE_PPROF = true
    ```

2. Restart Gitea

3. Try to trigger the bug, when the requests get stuck for a while,
   use `curl` or browser to visit: `http://127.0.0.1:6060/debug/pprof/goroutine?debug=1` to get the stacktrace.
