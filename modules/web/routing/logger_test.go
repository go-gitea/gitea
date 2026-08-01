// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitea.dev/modules/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type requestLogLevelRecorder struct {
	levels []log.Level
}

func (l *requestLogLevelRecorder) Log(_ int, event *log.Event, _ string, _ ...any) {
	l.levels = append(l.levels, event.Level)
}

func (l *requestLogLevelRecorder) GetLevel() log.Level {
	return log.TRACE
}

type requestStatusWriter struct {
	*httptest.ResponseRecorder
	status int
}

func (w *requestStatusWriter) WrittenStatus() int {
	return w.status
}

func TestRoutineControlPlaneRequestLogLevel(t *testing.T) {
	tests := []struct {
		name       string
		requestURI string
		status     int
		cancelled  bool
		want       log.Level
	}{
		{name: "actions fetch", requestURI: "/api/actions/runner.v1.RunnerService/FetchTask", status: http.StatusOK, want: log.TRACE},
		{name: "declare manager", requestURI: "/api/codespace/codespace.v1.ManagerService/DeclareManager", status: http.StatusOK, want: log.TRACE},
		{name: "fetch operations", requestURI: "/api/codespace/codespace.v1.ManagerService/FetchOperations", status: http.StatusOK, want: log.TRACE},
		{name: "report instances", requestURI: "/api/codespace/codespace.v1.ManagerService/ReportInstances", status: http.StatusOK, want: log.TRACE},
		{name: "update log", requestURI: "/api/codespace/codespace.v1.ManagerService/UpdateLog", status: http.StatusOK, want: log.TRACE},
		{name: "report runtime metadata", requestURI: "/api/codespace/codespace.v1.ManagerService/ReportRuntimeMetadata", status: http.StatusOK, want: log.TRACE},
		{name: "validate public endpoint", requestURI: "/api/codespace/codespace.v1.ManagerService/ValidatePublicEndpoint", status: http.StatusOK, want: log.TRACE},
		{name: "revalidate gateway session", requestURI: "/api/codespace/codespace.v1.ManagerService/RevalidateGatewaySession", status: http.StatusOK, want: log.TRACE},
		{name: "cancelled routine request", requestURI: "/api/codespace/codespace.v1.ManagerService/FetchOperations", cancelled: true, want: log.TRACE},
		{name: "routine request without response", requestURI: "/api/codespace/codespace.v1.ManagerService/FetchOperations", want: log.INFO},
		{name: "routine request error", requestURI: "/api/codespace/codespace.v1.ManagerService/FetchOperations", status: http.StatusBadRequest, want: log.INFO},
		{name: "lifecycle request", requestURI: "/api/codespace/codespace.v1.ManagerService/FinalizeOperation", status: http.StatusOK, want: log.INFO},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := &requestLogLevelRecorder{}
			request := httptest.NewRequest(http.MethodPost, test.requestURI, nil)
			if test.cancelled {
				ctx, cancel := context.WithCancel(request.Context())
				cancel()
				request = request.WithContext(ctx)
			}
			logPrinter(log.BaseLoggerToGeneralLogger(logger))(EndEvent, &requestRecord{
				startTime:  time.Now(),
				request:    request,
				respWriter: &requestStatusWriter{ResponseRecorder: httptest.NewRecorder(), status: test.status},
				funcInfo:   GetFuncInfo(http.NotFound),
			})
			require.Len(t, logger.levels, 1)
			assert.Equal(t, test.want, logger.levels[0])
		})
	}
}
