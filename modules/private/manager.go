// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// Shutdown calls the internal shutdown function
func Shutdown(ctx context.Context) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/manager/shutdown"
	req := newInternalRequest(ctx, reqURL, "POST")
	return requestJSONUserMsg(req, "Shutting down")
}

// Restart calls the internal restart function
func Restart(ctx context.Context) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/manager/restart"
	req := newInternalRequest(ctx, reqURL, "POST")
	return requestJSONUserMsg(req, "Restarting")
}

// FlushOptions represents the options for the flush call
type FlushOptions struct {
	Timeout     time.Duration
	NonBlocking bool
}

// FlushQueues calls the internal flush-queues function
func FlushQueues(ctx context.Context, timeout time.Duration, nonBlocking bool) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/manager/flush-queues"
	req := newInternalRequest(ctx, reqURL, "POST", FlushOptions{Timeout: timeout, NonBlocking: nonBlocking})
	if timeout > 0 {
		req.SetReadWriteTimeout(timeout + 10*time.Second)
	}
	return requestJSONUserMsg(req, "Flushed")
}

// PauseLogging pauses logging
func PauseLogging(ctx context.Context) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/manager/pause-logging"
	req := newInternalRequest(ctx, reqURL, "POST")
	return requestJSONUserMsg(req, "Logging Paused")
}

// ResumeLogging resumes logging
func ResumeLogging(ctx context.Context) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/manager/resume-logging"
	req := newInternalRequest(ctx, reqURL, "POST")
	return requestJSONUserMsg(req, "Logging Restarted")
}

// ReleaseReopenLogging releases and reopens logging files
func ReleaseReopenLogging(ctx context.Context) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/manager/release-and-reopen-logging"
	req := newInternalRequest(ctx, reqURL, "POST")
	return requestJSONUserMsg(req, "Logging Restarted")
}

// SetLogSQL sets database logging
func SetLogSQL(ctx context.Context, on bool) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/manager/set-log-sql?on=" + strconv.FormatBool(on)
	req := newInternalRequest(ctx, reqURL, "POST")
	return requestJSONUserMsg(req, "Log SQL setting set")
}

// LoggerOptions represents the options for the add logger call
type LoggerOptions struct {
	Group  string
	Name   string
	Mode   string
	Config map[string]interface{}
}

// AddLogger adds a logger
func AddLogger(ctx context.Context, group, name, mode string, config map[string]interface{}) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/manager/add-logger"
	req := newInternalRequest(ctx, reqURL, "POST", LoggerOptions{
		Group:  group,
		Name:   name,
		Mode:   mode,
		Config: config,
	})
	return requestJSONUserMsg(req, "Added")
}

// RemoveLogger removes a logger
func RemoveLogger(ctx context.Context, group, name string) ResponseExtra {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/manager/remove-logger/%s/%s", url.PathEscape(group), url.PathEscape(name))
	req := newInternalRequest(ctx, reqURL, "POST")
	return requestJSONUserMsg(req, "Removed")
}

// Processes return the current processes from this gitea instance
func Processes(ctx context.Context, out io.Writer, flat, noSystem, stacktraces, json bool, cancel string) ResponseExtra {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/manager/processes?flat=%t&no-system=%t&stacktraces=%t&json=%t&cancel-pid=%s", flat, noSystem, stacktraces, json, url.QueryEscape(cancel))

	req := newInternalRequest(ctx, reqURL, "GET")
	callback := func(resp *http.Response, extra *ResponseExtra) {
		_, extra.Error = io.Copy(out, resp.Body)
	}
	_, extra := requestJSONResp(req, &callback)
	return extra
}
