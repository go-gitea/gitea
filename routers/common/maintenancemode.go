// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"strings"

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

		// user login: "/user/login", "/user/logout", "/catpcha/..."
		"/user/",
		"/captcha/",
	}
	isMaintenanceModeAllowedRequest := func(req *http.Request) bool {
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(req.URL.Path, prefix) {
				return true
			}
		}
		if req.URL.Path == "/api/healthz" {
			return true
		}
		return false
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
