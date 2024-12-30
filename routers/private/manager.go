// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/graceful/releasereopen"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
)

// ReloadTemplates reloads all the templates
func ReloadTemplates(ctx *context.PrivateContext) {
	err := templates.ReloadHTMLTemplates()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			UserMsg: fmt.Sprintf("Template error: %v", err),
		})
		return
	}
	ctx.PlainText(http.StatusOK, "success")
}

// FlushQueues flushes all the Queues
func FlushQueues(ctx *context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.FlushOptions)
	if opts.NonBlocking {
		// Save the hammer ctx here - as a new one is created each time you call this.
		baseCtx := graceful.GetManager().HammerContext()
		go func() {
			err := queue.GetManager().FlushAll(baseCtx, opts.Timeout)
			if err != nil {
				log.Error("Flushing request timed-out with error: %v", err)
			}
		}()
		ctx.JSON(http.StatusAccepted, private.Response{
			UserMsg: "Flushing",
		})
		return
	}
	err := queue.GetManager().FlushAll(ctx, opts.Timeout)
	if err != nil {
		ctx.JSON(http.StatusRequestTimeout, private.Response{
			UserMsg: fmt.Sprintf("%v", err),
		})
	}
	ctx.PlainText(http.StatusOK, "success")
}

// PauseLogging pauses logging
func PauseLogging(ctx *context.PrivateContext) {
	log.GetManager().PauseAll()
	ctx.PlainText(http.StatusOK, "success")
}

// ResumeLogging resumes logging
func ResumeLogging(ctx *context.PrivateContext) {
	log.GetManager().ResumeAll()
	ctx.PlainText(http.StatusOK, "success")
}

// ReleaseReopenLogging releases and reopens logging files
func ReleaseReopenLogging(ctx *context.PrivateContext) {
	if err := releasereopen.GetManager().ReleaseReopen(); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Error during release and reopen: %v", err),
		})
		return
	}
	ctx.PlainText(http.StatusOK, "success")
}

// SetLogSQL re-sets database SQL logging
func SetLogSQL(ctx *context.PrivateContext) {
	db.SetLogSQL(ctx, ctx.FormBool("on"))
	ctx.PlainText(http.StatusOK, "success")
}

// RemoveLogger removes a logger
func RemoveLogger(ctx *context.PrivateContext) {
	logger := ctx.PathParam("logger")
	writer := ctx.PathParam("writer")
	err := log.GetManager().GetLogger(logger).RemoveWriter(writer)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to remove log writer: %s %s %v", logger, writer, err),
		})
		return
	}
	ctx.PlainText(http.StatusOK, fmt.Sprintf("Removed %s %s", logger, writer))
}

// AddLogger adds a logger
func AddLogger(ctx *context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.LoggerOptions)

	if len(opts.Logger) == 0 {
		opts.Logger = log.DEFAULT
	}

	writerMode := log.WriterMode{}
	writerType := opts.Mode

	var flags string
	var ok bool
	if flags, ok = opts.Config["flags"].(string); !ok {
		switch opts.Logger {
		case "access":
			flags = ""
		case "router":
			flags = "date,time"
		default:
			flags = "stdflags"
		}
	}
	writerMode.Flags = log.FlagsFromString(flags)

	if writerMode.Colorize, ok = opts.Config["colorize"].(bool); !ok && opts.Mode == "console" {
		if _, ok := opts.Config["stderr"]; ok {
			writerMode.Colorize = log.CanColorStderr
		} else {
			writerMode.Colorize = log.CanColorStdout
		}
	}

	writerMode.Level = setting.Log.Level
	if level, ok := opts.Config["level"].(string); ok {
		writerMode.Level = log.LevelFromString(level)
	}

	writerMode.StacktraceLevel = setting.Log.StacktraceLogLevel
	if stacktraceLevel, ok := opts.Config["level"].(string); ok {
		writerMode.StacktraceLevel = log.LevelFromString(stacktraceLevel)
	}

	writerMode.Prefix, _ = opts.Config["prefix"].(string)
	writerMode.Expression, _ = opts.Config["expression"].(string)

	switch writerType {
	case "console":
		writerOption := log.WriterConsoleOption{}
		writerOption.Stderr, _ = opts.Config["stderr"].(bool)
		writerMode.WriterOption = writerOption
	case "file":
		writerOption := log.WriterFileOption{}
		fileName, _ := opts.Config["filename"].(string)
		writerOption.FileName = setting.LogPrepareFilenameForWriter(fileName, opts.Writer+".log")
		writerOption.LogRotate, _ = opts.Config["rotate"].(bool)
		maxSizeShift, _ := opts.Config["maxsize"].(int)
		if maxSizeShift == 0 {
			maxSizeShift = 28
		}
		writerOption.MaxSize = 1 << maxSizeShift
		writerOption.DailyRotate, _ = opts.Config["daily"].(bool)
		writerOption.MaxDays, _ = opts.Config["maxdays"].(int)
		if writerOption.MaxDays == 0 {
			writerOption.MaxDays = 7
		}
		writerOption.Compress, _ = opts.Config["compress"].(bool)
		writerOption.CompressionLevel, _ = opts.Config["compressionLevel"].(int)
		if writerOption.CompressionLevel == 0 {
			writerOption.CompressionLevel = -1
		}
		writerMode.WriterOption = writerOption
	case "conn":
		writerOption := log.WriterConnOption{}
		writerOption.ReconnectOnMsg, _ = opts.Config["reconnectOnMsg"].(bool)
		writerOption.Reconnect, _ = opts.Config["reconnect"].(bool)
		writerOption.Protocol, _ = opts.Config["net"].(string)
		writerOption.Addr, _ = opts.Config["address"].(string)
		writerMode.WriterOption = writerOption
	default:
		panic(fmt.Sprintf("invalid log writer mode: %s", writerType))
	}
	writer, err := log.NewEventWriter(opts.Writer, writerType, writerMode)
	if err != nil {
		log.Error("Failed to create new log writer: %v", err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to create new log writer: %v", err),
		})
		return
	}
	log.GetManager().GetLogger(opts.Logger).AddWriters(writer)
	ctx.PlainText(http.StatusOK, "success")
}
