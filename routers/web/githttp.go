// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/repo"
	context_service "code.gitea.io/gitea/services/context"
)

func requireSignIn(ctx *context.Context) {
	if !setting.Service.RequireSignInView {
		return
	}

	// rely on the results of Contexter
	if !ctx.IsSigned {
		// TODO: support digit auth - which would be Authorization header with digit
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea"`)
		ctx.Error(http.StatusUnauthorized)
	}
}

func gitHTTPRouters(m *web.Route) {
	m.Group("", func() {
		m.PostOptions("/git-upload-pack", repo.ServiceUploadPack)
		m.PostOptions("/git-receive-pack", repo.ServiceReceivePack)
		m.GetOptions("/info/refs", repo.GetInfoRefs)
		m.GetOptions("/HEAD", repo.GetTextFile("HEAD"))
		m.GetOptions("/objects/info/alternates", repo.GetTextFile("objects/info/alternates"))
		m.GetOptions("/objects/info/http-alternates", repo.GetTextFile("objects/info/http-alternates"))
		m.GetOptions("/objects/info/packs", repo.GetInfoPacks)
		m.GetOptions("/objects/info/{file:[^/]*}", repo.GetTextFile(""))
		m.GetOptions("/objects/{head:[0-9a-f]{2}}/{hash:[0-9a-f]{38}}", repo.GetLooseObject)
		m.GetOptions("/objects/pack/pack-{file:[0-9a-f]{40}}.pack", repo.GetPackFile)
		m.GetOptions("/objects/pack/pack-{file:[0-9a-f]{40}}.idx", repo.GetIdxFile)
	}, ignSignInAndCsrf, requireSignIn, repo.HTTPGitEnabledHandler, repo.CorsHandler(), context_service.UserAssignmentWeb())
}
