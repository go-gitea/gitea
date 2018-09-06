// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/convert"

	macaron "gopkg.in/macaron.v1"
)

// CheckInternalToken check internal token is set
func CheckInternalToken(ctx *macaron.Context) {
	tokens := ctx.Req.Header.Get("Authorization")
	fields := strings.Fields(tokens)
	if len(fields) != 2 || fields[0] != "Bearer" || fields[1] != setting.InternalToken {
		ctx.Error(403)
	}
}

// UpdatePublicKey update publick key updates
func UpdatePublicKey(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	if err := models.UpdatePublicKeyUpdated(keyID); err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.PlainText(200, []byte("success"))
}

//TODO move on specific file
//GetRepositoryByOwnerAndName chainload to models.GetRepositoryByOwnerAndName
func GetRepositoryByOwnerAndName(ctx *macaron.Context) {
	//TODO use repo.Get(ctx *context.APIContext) ?
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, repo.APIFormat(models.AccessModeAdmin)) //TODO verify only use for internal but maybe a lower access is enough
}

//GetPublicKeyByID chainload to models.GetPublicKeyByID
func GetPublicKeyByID(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	key, err := models.GetPublicKeyByID(keyID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, convert.ToPublicKey("", key)) //TODO check if api link is needed
}

// RegisterRoutes registers all internal APIs routes to web application.
// These APIs will be invoked by internal commands for example `gitea serv` and etc.
func RegisterRoutes(m *macaron.Macaron) {
	m.Group("/", func() {
		m.Get("/ssh/:id", GetPublicKeyByID)
		m.Post("/ssh/:id/update", UpdatePublicKey)
		m.Post("/push/update", PushUpdate)
		m.Get("/protectedbranch/:pbid/:userid", CanUserPush)
		m.Get("/repo/:owner/:repo", GetRepositoryByOwnerAndName)
		m.Get("/branch/:id/*", GetProtectedBranchBy)
		m.Get("/repository/:rid", GetRepository)
		m.Get("/active-pull-request", GetActivePullRequest)
	}, CheckInternalToken)
}
