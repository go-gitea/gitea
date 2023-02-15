// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type routerLoggerOptions struct {
	req            *http.Request
	Identity       *string
	Start          *time.Time
	ResponseWriter http.ResponseWriter
	Ctx            map[string]interface{}
	RequestID      *string
}

var signedUserNameStringPointerKey interface{} = "signedUserNameStringPointerKey"

const keyOfRequestIDInTemplate = ".RequestID"

// According to:
// In OpenTracing and OpenTelemetry, the maximum length of trace id is 256 bits (32 bytes).
// MD5 output is 16 or 32 bytes.
// UUID output is 36 bytes (including four ‘-’)
// SHA1 output is 40 bytes
// So, we accept a Request ID with a maximum character length of 40
const maxRequestIDBtyeLength = 40

func parseRequestIDFromRequestHeader(req *http.Request) string {
	requestID := "-"
	for _, key := range setting.RequestIDHeaders {
		if req.Header.Get(key) != "" {
			requestID = req.Header.Get(key)
			break
		}
	}
	if len(requestID) > maxRequestIDBtyeLength {
		requestID = fmt.Sprintf("%s...", requestID[:maxRequestIDBtyeLength])
	}
	return requestID
}

// AccessLogger returns a middleware to log access logger
func AccessLogger() func(http.Handler) http.Handler {
	logger := log.GetLogger("access")
	logTemplate, _ := template.New("log").Parse(setting.AccessLogTemplate)
	needRequestID := strings.Contains(setting.AccessLogTemplate, keyOfRequestIDInTemplate)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			identity := "-"
			r := req.WithContext(context.WithValue(req.Context(), signedUserNameStringPointerKey, &identity))

			var requestID string
			if needRequestID {
				requestID = parseRequestIDFromRequestHeader(req)
			}

			next.ServeHTTP(w, r)
			rw := w.(ResponseWriter)

			buf := bytes.NewBuffer([]byte{})
			err := logTemplate.Execute(buf, routerLoggerOptions{
				req:            req,
				Identity:       &identity,
				Start:          &start,
				ResponseWriter: rw,
				Ctx: map[string]interface{}{
					"RemoteAddr": req.RemoteAddr,
					"Req":        req,
				},
				RequestID: &requestID,
			})
			if err != nil {
				log.Error("Could not set up chi access logger: %v", err.Error())
			}

			err = logger.SendLog(log.INFO, "", "", 0, buf.String(), "")
			if err != nil {
				log.Error("Could not set up chi access logger: %v", err.Error())
			}
		})
	}
}
