// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strings"
	"text/template"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
)

type accessLoggerTmplData struct {
	Identity       *string
	Start          *time.Time
	ResponseWriter struct {
		Status, Size int
	}
	Ctx       map[string]any
	RequestID *string
}

const keyOfRequestIDInTemplate = ".RequestID"

// According to:
// TraceId: A valid trace identifier is a 16-byte array with at least one non-zero byte
// MD5 output is 16 or 32 bytes: md5-bytes is 16, md5-hex is 32
// SHA1: similar, SHA1-bytes is 20, SHA1-hex is 40.
// UUID is 128-bit, 32 hex chars, 36 ASCII chars with 4 dashes
// So, we accept a Request ID with a maximum character length of 40
const maxRequestIDByteLength = 40

func parseRequestIDFromRequestHeader(req *http.Request) string {
	requestID := "-"
	for _, key := range setting.Log.RequestIDHeaders {
		if req.Header.Get(key) != "" {
			requestID = req.Header.Get(key)
			break
		}
	}
	if len(requestID) > maxRequestIDByteLength {
		requestID = fmt.Sprintf("%s...", requestID[:maxRequestIDByteLength])
	}
	return requestID
}

type accessLogRecorder struct {
	logger        log.BaseLogger
	logTemplate   *template.Template
	needRequestID bool
}

func (lr *accessLogRecorder) record(start time.Time, respWriter ResponseWriter, req *http.Request) {
	var requestID string
	if lr.needRequestID {
		requestID = parseRequestIDFromRequestHeader(req)
	}

	reqHost, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		reqHost = req.RemoteAddr
	}

	identity := "-"
	data := middleware.GetContextData(req.Context())
	if signedUser, ok := data[middleware.ContextDataKeySignedUser].(*user_model.User); ok {
		identity = signedUser.Name
	}
	buf := bytes.NewBuffer([]byte{})
	tmplData := accessLoggerTmplData{
		Identity: &identity,
		Start:    &start,
		Ctx: map[string]any{
			"RemoteAddr": req.RemoteAddr,
			"RemoteHost": reqHost,
			"Req":        req,
		},
		RequestID: &requestID,
	}
	tmplData.ResponseWriter.Status = respWriter.WrittenStatus()
	tmplData.ResponseWriter.Size = respWriter.WrittenSize()
	err = lr.logTemplate.Execute(buf, tmplData)
	if err != nil {
		log.Error("Could not execute access logger template: %v", err.Error())
	}

	lr.logger.Log(1, &log.Event{Level: log.INFO}, "%s", buf.String())
}

func newAccessLogRecorder() *accessLogRecorder {
	return &accessLogRecorder{
		logger:        log.GetLogger("access"),
		logTemplate:   template.Must(template.New("log").Parse(setting.Log.AccessLogTemplate)),
		needRequestID: len(setting.Log.RequestIDHeaders) > 0 && strings.Contains(setting.Log.AccessLogTemplate, keyOfRequestIDInTemplate),
	}
}

// AccessLogger returns a middleware to log access logger
func AccessLogger() func(http.Handler) http.Handler {
	recorder := newAccessLogRecorder()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, req)
			recorder.record(start, w.(ResponseWriter), req)
		})
	}
}
