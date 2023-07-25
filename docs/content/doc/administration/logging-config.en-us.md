---
date: "2019-04-02T17:06:00+01:00"
title: "Logging Configuration"
slug: "logging-config"
weight: 40
toc: false
draft: false
aliases:
  - /en-us/logging-configuration
menu:
  sidebar:
    parent: "administration"
    name: "Logging Configuration"
    weight: 40
    identifier: "logging-config"
---

# Logging Configuration

The logging configuration of Gitea mainly consists of 3 types of components:

- The `[log]` section for general configuration
- `[log.<mode-name>]` sections for the configuration of different log writers to output logs, aka: "writer mode", the mode name is also used as "writer name".
- The `[log]` section can also contain sub-logger configurations following the key schema `logger.<logger-name>.<CONFIG-KEY>`

There is a fully functional log output by default, so it is not necessary to define one.

**Table of Contents**

{{< toc >}}

## Collecting Logs for Help

To collect logs for help and issue report, see [Support Options]({{< relref "doc/help/support.en-us.md" >}}).

## The `[log]` section

Configuration of logging facilities in Gitea happen in the `[log]` section and its subsections.

In the top level `[log]` section the following configurations can be placed:

- `ROOT_PATH`: (Default: **%(GITEA_WORK_DIR)/log**): Base path for log files
- `MODE`: (Default: **console**) List of log outputs to use for the Default logger.
- `LEVEL`: (Default: **Info**) Least severe log events to persist, case-insensitive. Possible values are: `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Fatal`.
- `STACKTRACE_LEVEL`: (Default: **None**) For this and more severe events the stacktrace will be printed upon getting logged.

And it can contain the following sub-loggers:

- `logger.router.MODE`: (Default: **,**): List of log outputs to use for the Router logger.
- `logger.access.MODE`: (Default: **\<empty\>**)  List of log outputs to use for the Access logger. By default, the access logger is disabled.
- `logger.xorm.MODE`: (Default: **,**) List of log outputs to use for the XORM logger.

Setting a comma (`,`) to sub-logger's mode means making it use the default global `MODE`.

## Quick samples

### Default (empty) Configuration

The empty configuration is equivalent to default:

```ini
[log]
ROOT_PATH = %(GITEA_WORK_DIR)/log
MODE = console
LEVEL = Info
STACKTRACE_LEVEL = None
logger.router.MODE = ,
logger.xorm.MODE = ,
logger.access.MODE =

; this is the config options of "console" mode (used by MODE=console above)
[log.console]
MODE = console
FLAGS = stdflags
PREFIX =
COLORIZE = true
```

This is equivalent to sending all logs to the console, with default Golang log being sent to the console log too.

This is only a sample, and it is the default, do not need to write it into your configuration file.

### Disable Router logs and record some access logs to file

The Router logger is disabled, the access logs (>=Warn) goes into `access.log`:

```ini
[log]
logger.router.MODE =
logger.access.MODE = access-file

[log.access-file]
MODE = file
LEVEL = Warn
FILE_NAME = access.log
```

### Set different log levels for different modes

Default logs (>=Warn) goes into `gitea.log`, while Error logs goes into `file-error.log`:

```ini
[log]
LEVEL = Warn
MODE = file, file-error

; by default, the "file" mode will record logs to %(log.ROOT_PATH)/gitea.log, so we don't need to set it
; [log.file]

[log.file-error]
LEVEL = Error
FILE_NAME = file-error.log
```

## Log outputs (mode and writer)

Gitea provides the following log output writers:

- `console` - Log to `stdout` (or `stderr` if it is set in the config)
- `file` - Log to a file
- `conn` - Log to a socket (network or unix)

### Common configuration

Certain configuration is common to all modes of log output:

- `MODE` is the mode of the log output writer. It will default to the mode name in the ini section. Thus `[log.console]` will default to `MODE = console`.
- `LEVEL` is the lowest level that this output will log.
- `STACKTRACE_LEVEL` is the lowest level that this output will print a stacktrace.
- `COLORIZE` will default to `true` for `console` as described, otherwise it will default to `false`.

#### `EXPRESSION`

`EXPRESSION` represents a regular expression that log events must match to be logged by the output writer.
Either the log message, (with colors removed), must match or the `longfilename:linenumber:functionname` must match.
NB: the whole message or string doesn't need to completely match.

Please note this expression will be run in the writer's goroutine but not the logging event goroutine.

#### `FLAGS`

`FLAGS` represents the preceding logging context information that is
printed before each message. It is a comma-separated string set. The order of values does not matter.

It defaults to `stdflags` (= `date,time,medfile,shortfuncname,levelinitial`)

Possible values are:

- `none` or `,` - No flags.
- `date` - the date in the local time zone: `2009/01/23`.
- `time` - the time in the local time zone: `01:23:23`.
- `microseconds` - microsecond resolution: `01:23:23.123123`. Assumes  time.
- `longfile` - full file name and line number: `/a/b/c/d.go:23`.
- `shortfile` - final file name element and line number: `d.go:23`.
- `funcname` - function name of the caller: `runtime.Caller()`.
- `shortfuncname` - last part of the function name. Overrides `funcname`.
- `utc` - if date or time is set, use UTC rather than the local time zone.
- `levelinitial` - initial character of the provided level in brackets eg. `[I]` for info.
- `level` - level in brackets `[INFO]`.
- `gopid` - the Goroutine-PID of the context.
- `medfile` - last 20 characters of the filename - equivalent to `shortfile,longfile`.
- `stdflags` - equivalent to `date,time,medfile,shortfuncname,levelinitial`.

### Console mode

In this mode the logger will forward log messages to the stdout and
stderr streams attached to the Gitea process.

For loggers in console mode, `COLORIZE` will default to `true` if not
on windows, or the Windows terminal can be set into ANSI mode or is a
cygwin or Msys pipe.

Settings:

- `STDERR`: **false**: Whether the logger should print to `stderr` instead of `stdout`.

### File mode

In this mode the logger will save log messages to a file.

Settings:

- `FILE_NAME`: The file to write the log events to, relative to `ROOT_PATH`, Default to `%(ROOT_PATH)/gitea.log`. Exception: access log will default to `%(ROOT_PATH)/access.log`.
- `MAX_SIZE_SHIFT`: **28**: Maximum size shift of a single file. 28 represents 256Mb. For details see below.
- `LOG_ROTATE` **true**: Whether to rotate the log files. TODO: if false, will it delete instead on daily rotate, or do nothing?.
- `DAILY_ROTATE`: **true**: Whether to rotate logs daily.
- `MAX_DAYS`: **7**: Delete rotated log files after this number of days.
- `COMPRESS`: **true**: Whether to compress old log files by default with gzip.
- `COMPRESSION_LEVEL`: **-1**: Compression level. For details see below.

`MAX_SIZE_SHIFT` defines the maximum size of a file by left shifting 1 the given number of times (`1 << x`).
The exact behavior at the time of v1.17.3 can be seen [here](https://github.com/go-gitea/gitea/blob/v1.17.3/modules/setting/log.go#L185).

The useful values of `COMPRESSION_LEVEL` are from 1 to (and including) 9, where higher numbers mean better compression.
Beware that better compression might come with higher resource usage.
Must be preceded with a `-` sign.

### Conn mode

In this mode the logger will send log messages over a network socket.

Settings:

- `ADDR`: **:7020**: Sets the address to connect to.
- `PROTOCOL`: **tcp**: Set the protocol, either "tcp", "unix" or "udp".
- `RECONNECT`: **false**: Try to reconnect when connection is lost.
- `RECONNECT_ON_MSG`: **false**: Reconnect host for every single message.

### The "Router" logger

The Router logger logs the following message types when Gitea's route handlers work:

- `started` messages will be logged at TRACE level
- `polling`/`completed` routers will be logged at INFO. Exception: "/assets" static resource requests are also logged at TRACE.
- `slow` routers will be logged at WARN
- `failed` routers will be logged at WARN

### The "XORM" logger

To make XORM outputs SQL logs, the `LOG_SQL` in `[database]` section should also be set to `true`.

### The "Access" logger

The Access logger is a new logger since Gitea 1.9. It provides a NCSA
Common Log compliant log format. It's highly configurable but caution
should be taken when changing its template. The main benefit of this
logger is that Gitea can now log accesses in a standard log format so
standard tools may be used.

You can enable this logger using `logger.access.MODE = ...`.

If desired the format of the Access logger can be changed by changing
the value of the `ACCESS_LOG_TEMPLATE`.

Please note, the access logger will log at `INFO` level, setting the
`LEVEL` of this logger to `WARN` or above will result in no access logs.

#### The ACCESS_LOG_TEMPLATE

This value represents a go template. Its default value is

```tmpl
{{.Ctx.RemoteHost}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.URL.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}" "{{.Ctx.Req.UserAgent}}"`
```

The template is passed following options:

- `Ctx` is the `context.Context`
- `Identity` is the `SignedUserName` or `"-"` if the user is not logged in
- `Start` is the start time of the request
- `ResponseWriter` is the `http.ResponseWriter`

Caution must be taken when changing this template as it runs outside of
the standard panic recovery trap. The template should also be as simple
as it runs for every request.

## Releasing-and-Reopening, Pausing and Resuming logging

If you are running on Unix you may wish to release-and-reopen logs in order to use `logrotate` or other tools.
It is possible force Gitea to release and reopen it's logging files and connections by sending `SIGUSR1` to the
running process, or running `gitea manager logging release-and-reopen`.

Alternatively, you may wish to pause and resume logging - this can be accomplished through the use of the
`gitea manager logging pause` and `gitea manager logging resume` commands. Please note that whilst logging
is paused log events below INFO level will not be stored and only a limited number of events will be stored.
Logging may block, albeit temporarily, slowing Gitea considerably whilst paused - therefore it is
recommended that pausing only done for a very short period of time.

## Adding and removing logging whilst Gitea is running

It is possible to add and remove logging whilst Gitea is running using the `gitea manager logging add` and `remove` subcommands.
This functionality can only adjust running log systems and cannot be used to start the access or router loggers if they
were not already initialized. If you wish to start these systems you are advised to adjust the app.ini and (gracefully) restart
the Gitea service.

The main intention of these commands is to easily add a temporary logger to investigate problems on running systems where a restart
may cause the issue to disappear.

## Using `logrotate` instead of built-in log rotation

Gitea includes built-in log rotation, which should be enough for most deployments. However, if you instead want to use the `logrotate` utility:

- Disable built-in log rotation by setting `LOG_ROTATE` to `false` in your `app.ini`.
- Install `logrotate`.
- Configure `logrotate` to match your deployment requirements, see `man 8 logrotate` for configuration syntax details.
  In the `postrotate/endscript` block send Gitea a `USR1` signal via `kill -USR1` or `kill -10` to the `gitea` process itself,
  or run `gitea manager logging release-and-reopen` (with the appropriate environment).
  Ensure that your configurations apply to all files emitted by Gitea loggers as described in the above sections.
- Always do `logrotate /etc/logrotate.conf --debug` to test your configurations.
- If you are using docker and are running from outside the container you can use
  `docker exec -u $OS_USER $CONTAINER_NAME sh -c 'gitea manager logging release-and-reopen'`
  or `docker exec $CONTAINER_NAME sh -c '/bin/s6-svc -1 /etc/s6/gitea/'` or send `USR1` directly to the Gitea process itself.

The next `logrotate` jobs will include your configurations, so no restart is needed.
You can also immediately reload `logrotate` with `logrotate /etc/logrotate.conf --force`.
