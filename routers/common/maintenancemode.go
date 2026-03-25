// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/setting"
)

func MaintenanceModeHandler() func(h http.Handler) http.Handler {
	allowedPrefixes := []string{
		"/.well-known/",
		"/assets/",
		"/avatars/",

		// admin: "/-/admin"
		// general-purpose URLs: "/-/fetch-redirect", "/-/markup", etc.
		"/-/",

		// internal APIs
		"/api/internal/",

		// user login (for admin to login): "/user/login", "/user/logout", "/catpcha/..."
		"/user/",
		"/captcha/",
	}
	allowedPaths := container.SetOf(
		"/api/healthz",
	)
	isMaintenanceModeAllowedRequest := func(req *http.Request) bool {
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(req.URL.Path, prefix) {
				return true
			}
		}
		return allowedPaths.Contains(req.URL.Path)
	}

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
