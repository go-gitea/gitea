// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/binding"
	"gitea.com/macaron/macaron"
)

// CheckInternalToken check internal token is set
func CheckInternalToken(ctx *macaron.Context) {
	tokens := ctx.Req.Header.Get("Authorization")
	fields := strings.Fields(tokens)
	if len(fields) != 2 || fields[0] != "Bearer" || fields[1] != setting.InternalToken {
		log.Debug("Forbidden attempt to access internal url: Authorization header: %s", tokens)
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
	bind := binding.Bind

	m.Group("/", func() {
		m.Post("/ssh/authorized_keys", AuthorizedPublicKeyByContent)
		m.Post("/ssh/:id/update/:repoid", UpdatePublicKeyInRepo)
		m.Post("/hook/pre-receive/:owner/:repo", bind(private.HookOptions{}), HookPreReceive)
		m.Post("/hook/post-receive/:owner/:repo", bind(private.HookOptions{}), HookPostReceive)
		m.Post("/hook/set-default-branch/:owner/:repo/:branch", SetDefaultBranch)
		m.Get("/serv/none/:keyid", ServNoCommand)
		m.Get("/serv/command/:keyid/:owner/:repo", ServCommand)
	}, CheckInternalToken)
}
