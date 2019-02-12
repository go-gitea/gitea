// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

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
	ctx.JSON(200, repo)
}

//CheckUnitUser chainload to models.CheckUnitUser
func CheckUnitUser(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	userID := ctx.ParamsInt64(":userid")
	repo, err := models.GetRepositoryByID(repoID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	var user *models.User
	if userID > 0 {
		user, err = models.GetUserByID(userID)
		if err != nil {
			ctx.JSON(500, map[string]interface{}{
				"err": err.Error(),
			})
			return
		}
	}

	perm, err := models.GetUserRepoPermission(repo, user)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.JSON(200, perm.UnitAccessMode(models.UnitType(ctx.QueryInt("unitType"))))
}

// RegisterRoutes registers all internal APIs routes to web application.
// These APIs will be invoked by internal commands for example `gitea serv` and etc.
func RegisterRoutes(m *macaron.Macaron) {
	m.Group("/", func() {
		m.Get("/ssh/:id", GetPublicKeyByID)
		m.Get("/ssh/:id/user", GetUserByKeyID)
		m.Post("/ssh/:id/update", UpdatePublicKey)
		m.Post("/repositories/:repoid/keys/:keyid/update", UpdateDeployKey)
		m.Get("/repositories/:repoid/user/:userid/checkunituser", CheckUnitUser)
		m.Get("/repositories/:repoid/has-keys/:keyid", HasDeployKey)
		m.Get("/repositories/:repoid/keys/:keyid", GetDeployKey)
		m.Get("/repositories/:repoid/wiki/init", InitWiki)
		m.Post("/push/update", PushUpdate)
		m.Get("/protectedbranch/:pbid/:userid", CanUserPush)
		m.Get("/repo/:owner/:repo", GetRepositoryByOwnerAndName)
		m.Get("/branch/:id/*", GetProtectedBranchBy)
		m.Get("/repository/:rid", GetRepository)
		m.Get("/active-pull-request", GetActivePullRequest)
	}, CheckInternalToken)
}
