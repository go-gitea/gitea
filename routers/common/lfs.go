// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"

	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/lfs"
)

const RouterMockPointCommonLFS = "common-lfs"

func AddOwnerRepoGitLFSRoutes(m *web.Router, middlewares ...any) {
	// shared by web and internal routers
	m.Group("/{username}/{reponame}/info/lfs", func() {
		m.Post("/objects/batch", lfs.CheckAcceptMediaType, lfs.BatchHandler)
		m.Put("/objects/{oid}/{size}", lfs.UploadHandler)
		m.Get("/objects/{oid}/{filename}", lfs.DownloadHandler)
		m.Get("/objects/{oid}", lfs.DownloadHandler)
		m.Post("/verify", lfs.CheckAcceptMediaType, lfs.VerifyHandler)
		m.Group("/locks", func() {
			m.Get("/", lfs.GetListLockHandler)
			m.Post("/", lfs.PostLockHandler)
			m.Post("/verify", lfs.VerifyLockHandler)
			m.Post("/{lid}/unlock", lfs.UnLockHandler)
		}, lfs.CheckAcceptMediaType)
		m.Any("/*", http.NotFound)
	}, append([]any{web.RouterMockPoint(RouterMockPointCommonLFS)}, middlewares...)...)
}
