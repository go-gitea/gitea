---
date: "2018-05-21T15:00:00+00:00"
title: "Support Options"
slug: "seek-help"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "help"
    name: "Support Options"
    weight: 20
    identifier: "seek-help"
---

# Support Options

- [Discord](https://discord.gg/Gitea)
- [Discourse Forum](https://discourse.gitea.io/)

**NOTE:** When asking for support, it may be a good idea to have the following available so that the person helping has all the info they need:

1. Your `app.ini` (with any sensitive data scrubbed as necessary).
2. The Gitea logs, and any other appropriate log files for the situation.
    * The logs are likely to be outputted to console. If you need to collect logs from files, 
      you could copy the following config into your `app.ini` (remove all other `[log]` sections),
      then you can find the `*.log` files in Gitea's log directory (default: `%(GITEA_WORK_DIR)/log`).
    ```ini
    ; To show all SQL logs, you can also set LOG_SQL=true in the [database] section 
    [log]
    LEVEL=debug
    MODE=console,file
    ROUTER=console,file
    XORM=console,file
    ENABLE_XORM_LOG=true
    FILE_NAME=gitea.log
    [log.file.router]
    FILE_NAME=router.log
    [log.file.xorm]
    FILE_NAME=xorm.log
    ``` 
3. Any error messages you are seeing.
4. When possible, try to replicate the issue on [try.gitea.io](https://try.gitea.io) and include steps so that others can reproduce the issue.
    * This will greatly improve the chance that the root of the issue can be quickly discovered and resolved.
5. If you meet slow/hanging/deadlock problems, please report the stack trace when the problem occurs:
    1. Enable pprof in `app.ini` and restart Gitea
    ```
    [server]
    ENABLE_PPROF = true
    ```
    2. Trigger the bug, when Gitea gets stuck, use curl or browser to visit: `http://127.0.0.1:6060/debug/pprof/goroutine?debug=1` (IP is `127.0.0.1` and port is `6060`)
    3. Report the output (the stack trace doesn't contain sensitive data)

## Bugs

If you found a bug, please create an [issue on GitHub](https://github.com/go-gitea/gitea/issues).

## Chinese Support

Support for the Chinese language is provided at [Our discourse](https://discourse.gitea.io/c/5-category/5) or QQ Group 328432459.
