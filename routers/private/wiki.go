// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"code.gitea.io/gitea/models"

	macaron "gopkg.in/macaron.v1"
)

// InitWiki initilizes wiki via repo id
func InitWiki(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64("repoid")

	repo, err := models.GetRepositoryByID(repoID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	err = repo.InitWiki()
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.Status(202)
}
