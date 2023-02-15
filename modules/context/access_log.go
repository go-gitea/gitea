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

// According to OpenTracing and OpenTelemetry, the maximum length of trace id is 256 bits.
// (32 bytes = 32 * 8 = 256 bits)
// So we accept a Request ID with a maximum character length of 32
const maxRequestIDBtyeLength = 32

func parseRequestIDFromRequestHeader(req *http.Request) (string, string) {
	requestHeader := "-"
	requestID := "-"
	for _, key := range setting.RequestIDHeaders {
		if req.Header.Get(key) != "" {
			requestHeader = key
			requestID = req.Header.Get(key)
			break
		}
	}
	if len(requestID) > maxRequestIDBtyeLength {
		requestID = fmt.Sprintf("%s...", requestID[:maxRequestIDBtyeLength])
	}
	return requestHeader, requestID
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
				_, requestID = parseRequestIDFromRequestHeader(req)
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
