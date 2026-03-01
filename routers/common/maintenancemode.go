// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

func isMaintenanceModeAllowedRequest(req *http.Request) bool {
	if strings.HasPrefix(req.URL.Path, "/-/") {
		// URLs like "/-/admin", "/-/fetch-redirect" and "/-/markup" are still accessible in maintenance mode
		return true
	}
	if strings.HasPrefix(req.URL.Path, "/api/internal/") {
		// internal APIs should be allowed
		return true
	}
	if strings.HasPrefix(req.URL.Path, "/user/") {
		// URLs like "/user/signin" and "/user/signup" are still accessible in maintenance mode
		return true
	}
	if strings.HasPrefix(req.URL.Path, "/assets/") {
		return true
	}
	return false
}

func MaintenanceModeHandler() func(h http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			maintenanceMode := setting.Config().Instance.MaintenanceMode.Value(req.Context())
			if maintenanceMode.IsActive() && !isMaintenanceModeAllowedRequest(req) {
				renderServiceUnavailable(resp, req)
				return
			}
			next.ServeHTTP(resp, req)
		})
	}
}
