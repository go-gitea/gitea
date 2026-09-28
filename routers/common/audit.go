// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/reqctx"
	audit_service "gitea.dev/services/audit"
)

// AuditOrigin publishes the origin and the client address of the request, so
// audit events recorded while serving it are attributed to it.
func AuditOrigin(origin audit_model.Origin) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if store := reqctx.GetRequestDataStore(req.Context()); store != nil {
				audit_service.SetRequestInfo(store, origin, httplib.RemoteHost(req))
			}
			next.ServeHTTP(resp, req)
		})
	}
}
